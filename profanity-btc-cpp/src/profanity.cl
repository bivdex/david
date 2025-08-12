/* profanity.cl
 * ============
 * Contains multi-precision arithmetic functions and iterative elliptical point
 * addition which is the heart of profanity.
 *
 * Terminology
 * ===========
 * 
 *
 * Cutting corners
 * ===============
 * In some instances this code will produce the incorrect results. The elliptical
 * point addition does for example not properly handle the case of two points
 * sharing the same X-coordinate. The reason the code doesn't handle it properly
 * is because it is very unlikely to ever occur and the performance penalty for
 * doing it right is too severe. In the future I'll introduce a periodic check
 * after N amount of cycles that verifies the integrity of all the points to
 * make sure that even very unlikely event are at some point rectified.
 * 
 * Currently, if any of the points in the kernels experiences the unlikely event
 * of an error then that point is forever garbage and your runtime-performance
 * will in practice be (i*I-N) / (i*I). i and I here refers to the values given
 * to the program via the -i and -I switches (default values of 255 and 16384
 * respectively) and N is the number of errornous points.
 *
 * So if a single error occurs you'll lose 1/(i*I) of your performance. That's
 * around 0.00002%. The program will still report the same hashrate of course,
 * only that some of that work is entirely wasted on this errornous point.
 *
 * Initialization of main structure
 * ================================
 *
 * Iteration
 * =========
 *
 *
 * TODO
 * ====
 *   * Update comments to reflect new optimizations and structure
 *
 */

// 包含比特币地址生成头文件
#include "btc_addr_fixed.clh"

/* ------------------------------------------------------------------------ */
/* Multiprecision functions                                                 */
/* ------------------------------------------------------------------------ */
#define MP_WORDS 8
#define MP_BITS 32
#define bswap32(n) (rotate(n & 0x00FF00FF, 24U)|(rotate(n, 8U) & 0x00FF00FF))

typedef uint mp_word;
typedef struct {
	mp_word d[MP_WORDS];
} mp_number;

// mod              = 0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2f
__constant const mp_number mod              = { {0xfffffc2f, 0xfffffffe, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff} };

// tripleNegativeGx = 0x92c4cc831269ccfaff1ed83e946adeeaf82c096e76958573f2287becbb17b196
__constant const mp_number tripleNegativeGx = { {0xbb17b196, 0xf2287bec, 0x76958573, 0xf82c096e, 0x946adeea, 0xff1ed83e, 0x1269ccfa, 0x92c4cc83 } };

// doubleNegativeGy = 0x6f8a4b11b2b8773544b60807e3ddeeae05d0976eb2f557ccc7705edf09de52bf
__constant const mp_number doubleNegativeGy = { {0x09de52bf, 0xc7705edf, 0xb2f557cc, 0x05d0976e, 0xe3ddeeae, 0x44b60807, 0xb2b87735, 0x6f8a4b11} };

// negativeGy       = 0xb7c52588d95c3b9aa25b0403f1eef75702e84bb7597aabe663b82f6f04ef2777
__constant const mp_number negativeGy       = { {0x04ef2777, 0x63b82f6f, 0x597aabe6, 0x02e84bb7, 0xf1eef757, 0xa25b0403, 0xd95c3b9a, 0xb7c52588 } };


// Multiprecision subtraction. Underflow signalled via return value.
mp_word mp_sub(mp_number * const r, const mp_number * const a, const mp_number * const b) {
	mp_word t, c = 0;

	for (mp_word i = 0; i < MP_WORDS; ++i) {
		t = a->d[i] - b->d[i] - c;
		c = t > a->d[i] ? 1 : (t == a->d[i] ? c : 0);

		r->d[i] = t;
	}

	return c;
}

// Multiprecision subtraction of the modulus saved in mod. Underflow signalled via return value.
mp_word mp_sub_mod(mp_number * const r) {
	mp_number mod = { {0xfffffc2f, 0xfffffffe, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff} };

	mp_word t, c = 0;

	for (mp_word i = 0; i < MP_WORDS; ++i) {
		t = r->d[i] - mod.d[i] - c;
		c = t > r->d[i] ? 1 : (t == r->d[i] ? c : 0);

		r->d[i] = t;
	}

	return c;
}

// Multiprecision subtraction modulo M, M = mod.
// This function is often also used for additions by subtracting a negative number. I've chosen
// to do this because:
//   1. It's easier to re-use an already existing function
//   2. A modular addition would have more overhead since it has to determine if the result of
//      the addition (r) is in the gap M <= r < 2^256. This overhead doesn't exist in a
//      subtraction. We immediately know at the end of a subtraction if we had underflow
//      or not by inspecting the carry value. M refers to the modulus saved in variable mod.
void mp_mod_sub(mp_number * const r, const mp_number * const a, const mp_number * const b) {
	mp_word i, t, c = 0;

	for (i = 0; i < MP_WORDS; ++i) {
		t = a->d[i] - b->d[i] - c;
		c = t < a->d[i] ? 0 : (t == a->d[i] ? c : 1);

		r->d[i] = t;
	}

	if (c) {
		c = 0;
		for (i = 0; i < MP_WORDS; ++i) {
			r->d[i] += mod.d[i] + c;
			c = r->d[i] < mod.d[i] ? 1 : (r->d[i] == mod.d[i] ? c : 0);
		}
	}
}

// Multiprecision subtraction modulo M from a constant number.
// I made this in the belief that using constant address space instead of private address space for any
// constant numbers would lead to increase in performance. Judges are still out on this one.
void mp_mod_sub_const(mp_number * const r, __constant const mp_number * const a, const mp_number * const b) {
	mp_word i, t, c = 0;

	for (i = 0; i < MP_WORDS; ++i) {
		t = a->d[i] - b->d[i] - c;
		c = t < a->d[i] ? 0 : (t == a->d[i] ? c : 1);

		r->d[i] = t;
	}

	if (c) {
		c = 0;
		for (i = 0; i < MP_WORDS; ++i) {
			r->d[i] += mod.d[i] + c;
			c = r->d[i] < mod.d[i] ? 1 : (r->d[i] == mod.d[i] ? c : 0);
		}
	}
}

// Multiprecision subtraction modulo M of G_x from a number.
// Specialization of mp_mod_sub in hope of performance gain.
void mp_mod_sub_gx(mp_number * const r, const mp_number * const a) {
	mp_word i, t, c = 0;

	t = a->d[0] - 0x16f81798; c = t < a->d[0] ? 0 : (t == a->d[0] ? c : 1); r->d[0] = t;
	t = a->d[1] - 0x59f2815b - c; c = t < a->d[1] ? 0 : (t == a->d[1] ? c : 1); r->d[1] = t;
	t = a->d[2] - 0x2dce28d9 - c; c = t < a->d[2] ? 0 : (t == a->d[2] ? c : 1); r->d[2] = t;
	t = a->d[3] - 0x029bfcdb - c; c = t < a->d[3] ? 0 : (t == a->d[3] ? c : 1); r->d[3] = t;
	t = a->d[4] - 0xce870b07 - c; c = t < a->d[4] ? 0 : (t == a->d[4] ? c : 1); r->d[4] = t;
	t = a->d[5] - 0x55a06295 - c; c = t < a->d[5] ? 0 : (t == a->d[5] ? c : 1); r->d[5] = t;
	t = a->d[6] - 0xf9dcbbac - c; c = t < a->d[6] ? 0 : (t == a->d[6] ? c : 1); r->d[6] = t;
	t = a->d[7] - 0x79be667e - c; c = t < a->d[7] ? 0 : (t == a->d[7] ? c : 1); r->d[7] = t;

	if (c) {
		c = 0;
		for (i = 0; i < MP_WORDS; ++i) {
			r->d[i] += mod.d[i] + c;
			c = r->d[i] < mod.d[i] ? 1 : (r->d[i] == mod.d[i] ? c : 0);
		}
	}
}

// Multiprecision subtraction modulo M of G_y from a number.
// Specialization of mp_mod_sub in hope of performance gain.
void mp_mod_sub_gy(mp_number * const r, const mp_number * const a) {
	mp_word i, t, c = 0;

	t = a->d[0] - 0xfb10d4b8; c = t < a->d[0] ? 0 : (t == a->d[0] ? c : 1); r->d[0] = t;
	t = a->d[1] - 0x9c47d08f - c; c = t < a->d[1] ? 0 : (t == a->d[1] ? c : 1); r->d[1] = t;
	t = a->d[2] - 0xa6855419 - c; c = t < a->d[2] ? 0 : (t == a->d[2] ? c : 1); r->d[2] = t;
	t = a->d[3] - 0xfd17b448 - c; c = t < a->d[3] ? 0 : (t == a->d[3] ? c : 1); r->d[3] = t;
	t = a->d[4] - 0x0e1108a8 - c; c = t < a->d[4] ? 0 : (t == a->d[4] ? c : 1); r->d[4] = t;
	t = a->d[5] - 0x5da4fbfc - c; c = t < a->d[5] ? 0 : (t == a->d[5] ? c : 1); r->d[5] = t;
	t = a->d[6] - 0x26a3c465 - c; c = t < a->d[6] ? 0 : (t == a->d[6] ? c : 1); r->d[6] = t;
	t = a->d[7] - 0x483ada77 - c; c = t < a->d[7] ? 0 : (t == a->d[7] ? c : 1); r->d[7] = t;

	if (c) {
		c = 0;
		for (i = 0; i < MP_WORDS; ++i) {
			r->d[i] += mod.d[i] + c;
			c = r->d[i] < mod.d[i] ? 1 : (r->d[i] == mod.d[i] ? c : 0);
		}
	}
}

// Multiprecision addition. Overflow signalled via return value.
mp_word mp_add(mp_number * const r, const mp_number * const a) {
	mp_word c = 0;

	for (mp_word i = 0; i < MP_WORDS; ++i) {
		r->d[i] += a->d[i] + c;
		c = r->d[i] < a->d[i] ? 1 : (r->d[i] == a->d[i] ? c : 0);
	}

	return c;
}

// Multiprecision addition of the modulus saved in mod. Overflow signalled via return value.
mp_word mp_add_mod(mp_number * const r) {
	mp_word c = 0;

	for (mp_word i = 0; i < MP_WORDS; ++i) {
		r->d[i] += mod.d[i] + c;
		c = r->d[i] < mod.d[i] ? 1 : (r->d[i] == mod.d[i] ? c : 0);
	}

	return c;
}

// Multiprecision addition of two numbers with one extra word each. Overflow signalled via return value.
mp_word mp_add_more(mp_number * const r, mp_word * const extraR, const mp_number * const a, const mp_word * const extraA) {
	const mp_word c = mp_add(r, a);
	*extraR += *extraA + c;
	return *extraR < *extraA ? 1 : (*extraR == *extraA ? c : 0);
}

// Multiprecision greater than or equal (>=) operator
mp_word mp_gte(const mp_number * const a, const mp_number * const b) {
	mp_word l = 0, g = 0;

	for (mp_word i = 0; i < MP_WORDS; ++i) {
		if (a->d[i] < b->d[i]) l |= (1 << i);
		if (a->d[i] > b->d[i]) g |= (1 << i);
	}

	return g >= l;
}

// Bit shifts a number with an extra word to the right one step
void mp_shr_extra(mp_number * const r, mp_word * const e) {
	r->d[0] = (r->d[1] << 31) | (r->d[0] >> 1);
	r->d[1] = (r->d[2] << 31) | (r->d[1] >> 1);
	r->d[2] = (r->d[3] << 31) | (r->d[2] >> 1);
	r->d[3] = (r->d[4] << 31) | (r->d[3] >> 1);
	r->d[4] = (r->d[5] << 31) | (r->d[4] >> 1);
	r->d[5] = (r->d[6] << 31) | (r->d[5] >> 1);
	r->d[6] = (r->d[7] << 31) | (r->d[6] >> 1);
	r->d[7] = (*e << 31) | (r->d[7] >> 1);
	*e >>= 1;
}

// Bit shifts a number to the right one step
void mp_shr(mp_number * const r) {
	r->d[0] = (r->d[1] << 31) | (r->d[0] >> 1);
	r->d[1] = (r->d[2] << 31) | (r->d[1] >> 1);
	r->d[2] = (r->d[3] << 31) | (r->d[2] >> 1);
	r->d[3] = (r->d[4] << 31) | (r->d[3] >> 1);
	r->d[4] = (r->d[5] << 31) | (r->d[4] >> 1);
	r->d[5] = (r->d[6] << 31) | (r->d[5] >> 1);
	r->d[6] = (r->d[7] << 31) | (r->d[6] >> 1);
	r->d[7] >>= 1;
}

// Multiplies a number with a word and adds it to an existing number with an extra word, overflow of the extra word is signalled in return value
// This is a special function only used for modular multiplication
mp_word mp_mul_word_add_extra(mp_number * const r, const mp_number * const a, const mp_word w, mp_word * const extra) {
	mp_word cM = 0; // Carry for multiplication
	mp_word cA = 0; // Carry for addition
	mp_word tM = 0; // Temporary storage for multiplication

	for (mp_word i = 0; i < MP_WORDS; ++i) {
		tM = (a->d[i] * w + cM);
		cM = mul_hi(a->d[i], w) + (tM < cM);

		r->d[i] += tM + cA;
		cA = r->d[i] < tM ? 1 : (r->d[i] == tM ? cA : 0);
	}

	*extra += cM + cA;
	return *extra < cM ? 1 : (*extra == cM ? cA : 0);
}

// Multiplies a number with a word, potentially adds modhigher to it, and then subtracts it from en existing number, no extra words, no overflow
// This is a special function only used for modular multiplication
void mp_mul_mod_word_sub(mp_number * const r, const mp_word w, const bool withModHigher) {
	// Having these numbers declared here instead of using the global values in __constant address space seems to lead
	// to better optimizations by the compiler on my GTX 1070.
	mp_number mod = { { 0xfffffc2f, 0xfffffffe, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff} };
	mp_number modhigher = { {0x00000000, 0xfffffc2f, 0xfffffffe, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff} };

	mp_word cM = 0; // Carry for multiplication
	mp_word cS = 0; // Carry for subtraction
	mp_word tS = 0; // Temporary storage for subtraction
	mp_word tM = 0; // Temporary storage for multiplication
	mp_word cA = 0; // Carry for addition of modhigher

	for (mp_word i = 0; i < MP_WORDS; ++i) {
		tM = (mod.d[i] * w + cM);
		cM = mul_hi(mod.d[i], w) + (tM < cM);

		tM += (withModHigher ? modhigher.d[i] : 0) + cA;
		cA = tM < (withModHigher ? modhigher.d[i] : 0) ? 1 : (tM == (withModHigher ? modhigher.d[i] : 0) ? cA : 0);

		tS = r->d[i] - tM - cS;
		cS = tS > r->d[i] ? 1 : (tS == r->d[i] ? cS : 0);

		r->d[i] = tS;
	}
}

// Modular multiplication. Based on Algorithm 3 (and a series of hunches) from this article:
// https://www.esat.kuleuven.be/cosic/publications/article-1191.pdf
// When I first implemented it I never encountered a situation where the additional end steps
// of adding or subtracting the modulo was necessary. Maybe it's not for the particular modulo
// used in secp256k1, maybe the overflow bit can be skipped in to avoid 8 subtractions and
// trade it for the final steps? Maybe the final steps are necessary but seldom needed?
// I have no idea, for the time being I'll leave it like this, also see the comments at the
// beginning of this document under the title "Cutting corners".
void mp_mod_mul(mp_number * const r, const mp_number * const X, const mp_number * const Y) {
	mp_number Z = { {0} };
	mp_word extraWord;

	for (int i = MP_WORDS - 1; i >= 0; --i) {
		// Z = Z * 2^32
		extraWord = Z.d[7]; Z.d[7] = Z.d[6]; Z.d[6] = Z.d[5]; Z.d[5] = Z.d[4]; Z.d[4] = Z.d[3]; Z.d[3] = Z.d[2]; Z.d[2] = Z.d[1]; Z.d[1] = Z.d[0]; Z.d[0] = 0;

		// Z = Z + X * Y_i
		bool overflow = mp_mul_word_add_extra(&Z, X, Y->d[i], &extraWord);

		// Z = Z - qM
		mp_mul_mod_word_sub(&Z, extraWord, overflow);
	}

	*r = Z;
}

// Modular inversion of a number. 
void mp_mod_inverse(mp_number * const r) {
	mp_number A = { { 1 } };
	mp_number C = { { 0 } };
	mp_number v = mod;

	mp_word extraA = 0;
	mp_word extraC = 0;

	while (r->d[0] || r->d[1] || r->d[2] || r->d[3] || r->d[4] || r->d[5] || r->d[6] || r->d[7]) {
		while (!(r->d[0] & 1)) {
			mp_shr(r);
			if (A.d[0] & 1) {
				extraA += mp_add_mod(&A);
			}

			mp_shr_extra(&A, &extraA);
		}

		while (!(v.d[0] & 1)) {
			mp_shr(&v);
			if (C.d[0] & 1) {
				extraC += mp_add_mod(&C);
			}

			mp_shr_extra(&C, &extraC);
		}

		if (mp_gte(r, &v)) {
			mp_sub(r, r, &v);
			mp_add_more(&A, &extraA, &C, &extraC);
		}
		else {
			mp_sub(&v, &v, r);
			mp_add_more(&C, &extraC, &A, &extraA);
		}
	}

	while (extraC) {
		extraC -= mp_sub_mod(&C);
	}

	v = mod;
	mp_sub(r, &v, &C);
}

/* ------------------------------------------------------------------------ */
/* Elliptic point and addition (with caveats).                              */
/* ------------------------------------------------------------------------ */
typedef struct {
	mp_number x;
	mp_number y;
} point;

// Elliptical point addition
// Does not handle points sharing X coordinate, this is a deliberate design choice.
// For more information on this choice see the beginning of this file.
void point_add(point * const r, point * const p, point * const o) {
	mp_number tmp;
	mp_number newX;
	mp_number newY;

	mp_mod_sub(&tmp, &o->x, &p->x);

	mp_mod_inverse(&tmp);

	mp_mod_sub(&newX, &o->y, &p->y);
	mp_mod_mul(&tmp, &tmp, &newX);

	mp_mod_mul(&newX, &tmp, &tmp);
	mp_mod_sub(&newX, &newX, &p->x);
	mp_mod_sub(&newX, &newX, &o->x);

	mp_mod_sub(&newY, &p->x, &newX);
	mp_mod_mul(&newY, &newY, &tmp);
	mp_mod_sub(&newY, &newY, &p->y);

	r->x = newX;
	r->y = newY;
}

/* ------------------------------------------------------------------------ */
/* Profanity.                                                               */
/* ------------------------------------------------------------------------ */
typedef struct {
	uint found;
	uint foundId;
	uchar foundHash[20];
} result;

void profanity_init_seed(__global const point * const precomp, point * const p, bool * const pIsFirst, const size_t precompOffset, const ulong seed) {
	point o;

	for (uchar i = 0; i < 8; ++i) {
		const uchar shift = i * 8;
		const uchar byte = (seed >> shift) & 0xFF;

		if (byte) {
			o = precomp[precompOffset + i * 255 + byte - 1];
			if (*pIsFirst) {
				*p = o;
				*pIsFirst = false;
			}
			else {
				point_add(p, p, &o);
			}
		}
	}
}

__kernel void profanity_init(__global const point * const precomp, __global mp_number * const pDeltaX, __global mp_number * const pPrevLambda, __global result * const pResult, const ulong4 seed) {
	const size_t id = get_global_id(0);
	point p;
	bool bIsFirst = true;

	mp_number tmp1, tmp2;
	point tmp3;

	// Calculate G^k where k = seed.wzyx (in other words, find the point indicated by the private key represented in seed)
	profanity_init_seed(precomp, &p, &bIsFirst, 8 * 255 * 0, seed.x);
	profanity_init_seed(precomp, &p, &bIsFirst, 8 * 255 * 1, seed.y);
	profanity_init_seed(precomp, &p, &bIsFirst, 8 * 255 * 2, seed.z);
	profanity_init_seed(precomp, &p, &bIsFirst, 8 * 255 * 3, seed.w + id);

	// Calculate current lambda in this point
	mp_mod_sub_gx(&tmp1, &p.x);
	mp_mod_inverse(&tmp1);

	mp_mod_sub_gy(&tmp2, &p.y); 
	mp_mod_mul(&tmp1, &tmp1, &tmp2);

	// Jump to next point (precomp[0] is the generator point G)
	tmp3 = precomp[0];
	point_add(&p, &tmp3, &p);

	// pDeltaX should contain the delta (x - G_x)
	mp_mod_sub_gx(&p.x, &p.x);

	pDeltaX[id] = p.x;
	pPrevLambda[id] = tmp1;

	for (uchar i = 0; i < PROFANITY_MAX_SCORE + 1; ++i) {
		pResult[i].found = 0;
	}
}

// This kernel calculates several modular inversions at once with just one inverse.
// It's an implementation of Algorithm 2.11 from Modern Computer Arithmetic:
// https://members.loria.fr/PZimmermann/mca/pub226.html 
//
// My RX 480 is very sensitive to changes in the second loop and sometimes I have
// to make seemingly non-functional changes to the code to make the compiler
// generate the most optimized version.
__kernel void profanity_inverse(__global const mp_number * const pDeltaX, __global mp_number * const pInverse) {
	const size_t id = get_global_id(0) * PROFANITY_INVERSE_SIZE;

	// negativeDoubleGy = 0x6f8a4b11b2b8773544b60807e3ddeeae05d0976eb2f557ccc7705edf09de52bf
	mp_number negativeDoubleGy = { {0x09de52bf, 0xc7705edf, 0xb2f557cc, 0x05d0976e, 0xe3ddeeae, 0x44b60807, 0xb2b87735, 0x6f8a4b11 } };

	mp_number copy1, copy2;
	mp_number buffer[PROFANITY_INVERSE_SIZE];
	mp_number buffer2[PROFANITY_INVERSE_SIZE];

	// We initialize buffer and buffer2 such that:
	// buffer[i] = pDeltaX[id] * pDeltaX[id + 1] * pDeltaX[id + 2] * ... * pDeltaX[id + i]
	// buffer2[i] = pDeltaX[id + i]
	buffer[0] = pDeltaX[id];
	for (uint i = 1; i < PROFANITY_INVERSE_SIZE; ++i) {
		buffer2[i] = pDeltaX[id + i];
		mp_mod_mul(&buffer[i], &buffer2[i], &buffer[i - 1]);
	}

	// Take the inverse of all x-values combined
	copy1 = buffer[PROFANITY_INVERSE_SIZE - 1];
	mp_mod_inverse(&copy1);

	// We multiply in -2G_y together with the inverse so that we have:
	//            - 2 * G_y
	//  ----------------------------
	//  x_0 * x_1 * x_2 * x_3 * ...
	mp_mod_mul(&copy1, &copy1, &negativeDoubleGy);

	// Multiply out each individual inverse using the buffers
	for (uint i = PROFANITY_INVERSE_SIZE - 1; i > 0; --i) {
		mp_mod_mul(&copy2, &copy1, &buffer[i - 1]);
		mp_mod_mul(&copy1, &copy1, &buffer2[i]);
		pInverse[id + i] = copy2;
	}

	pInverse[id] = copy1;
}

// This kernel performs en elliptical curve point addition. See:
// https://en.wikipedia.org/wiki/Elliptic_curve_point_multiplication#Point_addition
// I've made one mathematical optimization by never calculating x_r,
// instead I directly calculate the delta (x_q - x_p). It's for this
// delta we calculate the inverse and that's already been done at this
// point. By calculating and storing the next delta we don't have to
// calculate the delta in profanity_inverse_multiple which saves us
// one call to mp_mod_sub per point, but inversely we have to introduce
// an addition (or addition by subtracting a negative number) in
// profanity_end to retrieve the actual x-coordinate instead of the
// delta as that's what used for calculating the public hash.
//
// One optimization is when calculating the next y-coordinate. As
// given in the wiki the next y-coordinate is given by:
//   y_r = λ²(x_p - x_r) - y_p
// In our case the other point P is the generator point so x_p = G_x,
// a constant value. x_r is the new point which we never calculate, we
// calculate the new delta (x_q - x_p) instead. Let's denote the delta
// with d and new delta as d' and remove notation for points P and Q and
// instead refeer to x_p as G_x, y_p as G_y and x_q as x, y_q as y.
// Furthermore let's denote new x by x' and new y with y'.
//
// Then we have:
//   d = x - G_x <=> x = d + G_x
//   x' = λ² - G_x - x <=> x_r = λ² - G_x - d - G_x = λ² - 2G_x - d
//   
//   d' = x' - G_x = λ² - 2G_x - d - G_x = λ² - 3G_x - d
//
// So we see that the new delta d' can be calculated with the same
// amount of steps as the new x'; 3G_x is still just a single constant.
//
// Now for the next y-coordinate in the new notation:
//   y' =  λ(G_x - x') - G_y
//
// If we expand the expression (G_x - x') we can see that this
// subtraction can be removed! Saving us one call to mp_mod_sub!
//   G_x - x' = -(x' - G_x) = -d'
// It has the same value as the new delta but negated! We can avoid
// having to perform the negation by:
//   y' = λ * -d' - G_y = -G_y - (λ * d')
//
// We can just precalculate the constant -G_y and we get rid of one
// subtraction. Woo!
//
// But we aren't done yet! Let's expand the expression for the next
// lambda, λ'. We have:
//   λ' = (y' - G_y) / d'
//      = (-λ * d' - G_y - G_y) / d' 
//      = (-λ * d' - 2*G_y) / d' 
//      = -λ - 2*G_y / d' 
//
// So the next lambda value can be calculated from the old one. This in
// and of itself is not so interesting but the fact that the term -2 * G_y
// is a constant is! Since it's constant it'll be the same value no matter
// which point we're currently working with. This means that this factor
// can be multiplied in during the inversion, and just with one call per
// inversion instead of one call per point! This is small enough to be
// negligible and thus we've reduced our point addition from three
// multi-precision multiplications to just two! Wow. Just wow.
//
// There is additional overhead introduced by storing the previous lambda
// but it's still a net gain. To additionally decrease memory access
// overhead I never any longer store the Y coordinate. Instead I
// calculate it at the end directly from the lambda and deltaX.
// 
// In addition to this some algebraic re-ordering has been done to move
// constants into the same argument to a new function mp_mod_sub_const
// in hopes that using constant storage instead of private storage
// will aid speeds.
//
// After the above point addition this kernel calculates the public address
// corresponding to the point and stores it in pInverse which is used only
// as interim storage as it won't otherwise be used again this cycle.
//
// One of the scoring kernels will run after this and fetch the address
// from pInverse.
__kernel void profanity_iterate(__global mp_number * const pDeltaX, __global mp_number * const pInverse, __global mp_number * const pPrevLambda) {
	const size_t id = get_global_id(0);

	// negativeGx = 0x8641998106234453aa5f9d6a3178f4f8fd640324d231d726a60d7ea3e907e497
	mp_number negativeGx = { {0xe907e497, 0xa60d7ea3, 0xd231d726, 0xfd640324, 0x3178f4f8, 0xaa5f9d6a, 0x06234453, 0x86419981 } };

	ethhash h = { { 0 } };

	mp_number dX = pDeltaX[id];
	mp_number tmp = pInverse[id];
	mp_number lambda = pPrevLambda[id];

	// λ' = - (2G_y) / d' - λ <=> lambda := pInversedNegativeDoubleGy[id] - pPrevLambda[id]
	mp_mod_sub(&lambda, &tmp, &lambda);

	// λ² = λ * λ <=> tmp := lambda * lambda = λ²
	mp_mod_mul(&tmp, &lambda, &lambda);

	// d' = λ² - d - 3g = (-3g) - (d - λ²) <=> x := tripleNegativeGx - (x - tmp)
	mp_mod_sub(&dX, &dX, &tmp);
	mp_mod_sub_const(&dX, &tripleNegativeGx, &dX);

	pDeltaX[id] = dX;
	pPrevLambda[id] = lambda;

	// Calculate y from dX and lambda
	// y' = (-G_Y) - λ * d' <=> p.y := negativeGy - (p.y * p.x)
	mp_mod_mul(&tmp, &lambda, &dX);
	mp_mod_sub_const(&tmp, &negativeGy, &tmp);

	// Restore X coordinate from delta value
	mp_mod_sub(&dX, &dX, &negativeGx);

	// 改进的比特币地址类型随机选择 (0-3)
	// 使用更好的随机性来源
	uchar addressType = (id + (uint)(get_global_id(0) * 0x1234567) + (uint)(get_global_id(0) >> 8)) % 4;
	
	// 准备公钥数据（压缩格式）
	__private uchar pubkey[33];
	pubkey[0] = (dX.d[0] & 1) ? 0x03 : 0x02; // 压缩标志
	
	// 修复：正确复制 X 坐标到公钥（不需要字节序转换）
	// dX.d 已经是正确的字节序，直接复制即可
	for (int i = 0; i < 8; i++) {
		uint x_val = dX.d[7-i];  // 修复：移除错误的bswap32
		pubkey[1 + i*4] = (x_val >> 24) & 0xFF;
		pubkey[2 + i*4] = (x_val >> 16) & 0xFF;
		pubkey[3 + i*4] = (x_val >> 8) & 0xFF;
		pubkey[4 + i*4] = x_val & 0xFF;
	}
	
	// 生成比特币地址
	__private uchar hash160[20];
	__private uchar script_hash[32];
	__private char btc_address[92];
	
	// 先计算hash160，避免重复调用
	hash160_pubkey((__global const uchar*)pubkey, 33, (__global uchar*)hash160);
	
	switch (addressType) {
		case 0: { // Legacy P2PKH
			p2pkh_to_base58((__global const uchar*)hash160, btc_address);
			break;
		}
		case 1: { // SegWit P2SH
			// 创建 P2WPKH 脚本: OP_0 + OP_PUSH(20) + hash160
			__private uchar script[22];
			script[0] = 0x00; // OP_0
			script[1] = 0x14; // OP_PUSH(20)
			for (int i = 0; i < 20; i++) {
				script[2 + i] = hash160[i];
			}
			hash160_script((__global const uchar*)script, 22, (__global uchar*)hash160);
			p2sh_to_base58((__global const uchar*)hash160, btc_address);
			break;
		}
		case 2: { // Native SegWit P2WPKH
			make_p2wpkh_bech32((__global const uchar*)hash160, 20, btc_address);
			break;
		}
		case 3: { // Taproot P2TR (简化版本，实际应该使用 Schnorr 签名)
			// 这里使用 P2WSH 作为替代，因为完整的 Taproot 实现比较复杂
			// 创建一个简单的脚本哈希
			sha256((__global const uchar*)hash160, 20, (__global uchar*)script_hash);
			make_p2wsh_bech32((__global const uchar*)script_hash, 32, btc_address);
			break;
		}
	}
	
	// 保存 hash160 到 pInverse (前20字节)
	pInverse[id].d[0] = ((uint)hash160[0] << 24) | ((uint)hash160[1] << 16) | ((uint)hash160[2] << 8) | hash160[3];
	pInverse[id].d[1] = ((uint)hash160[4] << 24) | ((uint)hash160[5] << 16) | ((uint)hash160[6] << 8) | hash160[7];
	pInverse[id].d[2] = ((uint)hash160[8] << 24) | ((uint)hash160[9] << 16) | ((uint)hash160[10] << 8) | hash160[11];
	pInverse[id].d[3] = ((uint)hash160[12] << 24) | ((uint)hash160[13] << 16) | ((uint)hash160[14] << 8) | hash160[15];
	pInverse[id].d[4] = ((uint)hash160[16] << 24) | ((uint)hash160[17] << 16) | ((uint)hash160[18] << 8) | hash160[19];
	
	// 在剩余的空间中存储地址类型信息
	// 将addressType存储到d[5]的低8位
	pInverse[id].d[5] = addressType;
	
	// 修复：扩展地址存储空间，使用更多的uint来存储完整地址
	// 计算地址长度
	int addrLen = 0;
	while (btc_address[addrLen] != 0 && addrLen < 92) {
		addrLen++;
	}
	
	// 修复：由于MP_WORDS只有8，我们需要使用更高效的存储策略
	// 使用d[6]和d[7]来存储地址，但采用压缩编码
	// 每个字符使用6位编码（64个字符足够表示Base58和Bech32字符集）
	// 这样每个uint可以存储5个字符（30位/6位 = 5字符）
	
	// 清空d[6]和d[7]
	pInverse[id].d[6] = 0;
	pInverse[id].d[7] = 0;
	
	// 压缩编码：将字符映射到6位值
	__private uchar char_to_6bit[128];
	for (int i = 0; i < 128; i++) {
		char_to_6bit[i] = 0xFF; // 默认无效值
	}
	
	// 初始化Base58字符集映射
	__private char base58_chars[] = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";
	for (int i = 0; i < 58; i++) {
		char_to_6bit[(int)base58_chars[i]] = i;
	}
	
	// 初始化Bech32字符集映射
	__private char bech32_chars[] = "qpzry9x8gf2tvdw0s3jn54khce6mua7l";
	for (int i = 0; i < 32; i++) {
		char_to_6bit[(int)bech32_chars[i]] = i + 58; // 从58开始，避免冲突
	}
	
	// 压缩编码到d[6]和d[7]
	uint packed6 = 0, packed7 = 0;
	int bit_pos6 = 0, bit_pos7 = 0;
	
	for (int i = 0; i < addrLen && i < 10; i++) { // 最多存储10个字符（5个在d[6]，5个在d[7]）
		uchar char_val = char_to_6bit[(int)btc_address[i]];
		if (char_val == 0xFF) {
			char_val = 0; // 无效字符映射为0
		}
		
		if (i < 5) {
			// 存储到d[6]
			packed6 |= ((uint)char_val) << bit_pos6;
			bit_pos6 += 6;
		} else {
			// 存储到d[7]
			packed7 |= ((uint)char_val) << bit_pos7;
			bit_pos7 += 6;
		}
	}
	
	pInverse[id].d[6] = packed6;
	pInverse[id].d[7] = packed7;
}

// 通用的BTC地址解码函数，用于所有评分函数
inline void decode_btc_address_from_pinverse(__global const mp_number * const pInverse, const size_t id, __private char *btc_address) {
	// 地址字符串被压缩编码在d[6]和d[7]中，使用6位编码
	// 需要解码压缩编码的地址
	__private char base58_chars[] = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";
	__private char bech32_chars[] = "qpzry9x8gf2tvdw0s3jn54khce6mua7l";
	
	int addrIndex = 0;
	uint packed6 = pInverse[id].d[6];
	uint packed7 = pInverse[id].d[7];
	
	// 解码d[6]中的前5个字符
	for (int i = 0; i < 5 && addrIndex < 92; i++) {
		uchar char_val = (packed6 >> (i * 6)) & 0x3F; // 6位掩码
		if (char_val == 0) break; // 遇到0表示结束
		
		char decoded_char;
		if (char_val < 58) {
			// Base58字符
			decoded_char = base58_chars[char_val];
		} else if (char_val < 90) {
			// Bech32字符
			decoded_char = bech32_chars[char_val - 58];
		} else {
			decoded_char = '?'; // 无效字符
		}
		
		btc_address[addrIndex++] = decoded_char;
	}
	
	// 解码d[7]中的后5个字符
	for (int i = 0; i < 5 && addrIndex < 92; i++) {
		uchar char_val = (packed7 >> (i * 6)) & 0x3F; // 6位掩码
		if (char_val == 0) break; // 遇到0表示结束
		
		char decoded_char;
		if (char_val < 58) {
			// Base58字符
			decoded_char = base58_chars[char_val];
		} else if (char_val < 90) {
			// Bech32字符
			decoded_char = bech32_chars[char_val - 58];
		} else {
			decoded_char = '?'; // 无效字符
		}
		
		btc_address[addrIndex++] = decoded_char;
	}
	
	// 确保字符串以null结尾
	if (addrIndex < 92) {
		btc_address[addrIndex] = '\0';
	}
}

void profanity_result_update(const size_t id, __global const uchar * const hash, __global result * const pResult, const uchar score, const uchar scoreMax, __global const mp_number * const pInverse) {
	if (score > 0 && score <= scoreMax) {
		uchar hasResult = atomic_inc(&pResult[score].found); // NOTE: If "too many" results are found it'll wrap around to 0 again and overwrite last result. Only relevant if global worksize exceeds MAX(uint).

		// Save only one result for each score, the first.
		if (hasResult == 0) {
			pResult[score].foundId = id;

			// 保存hash160
			for (int i = 0; i < 20; ++i) {
				pResult[score].foundHash[i] = hash[i];
			}
			
			// 保存地址类型
			pResult[score].addressType = pInverse[id].d[5] & 0xFF;
			
			// 从pInverse中提取完整的BTC地址字符串
			__private char btc_address[92];
			decode_btc_address_from_pinverse(pInverse, id, btc_address);
			
			// 将BTC地址复制到pResult结构体中
			for (int i = 0; i < 92; ++i) {
				pResult[score].btcAddress[i] = btc_address[i];
			}
		}
	}
}

__kernel void profanity_transform_contract(__global mp_number * const pInverse) {
	const size_t id = get_global_id(0);
	__global const uchar * const hash = pInverse[id].d;

	ethhash h;
	for (int i = 0; i < 50; ++i) {
		h.d[i] = 0;
	}
	// set up keccak(0xd6, 0x94, address, 0x80)
	h.b[0] = 214;
	h.b[1] = 148;
	for (int i = 0; i < 20; i++) {
		h.b[i + 2] = hash[i];
	}
	h.b[22] = 128;

	h.b[23] ^= 0x01; // length 23
	sha3_keccakf(&h);

	pInverse[id].d[0] = h.d[3];
	pInverse[id].d[1] = h.d[4];
	pInverse[id].d[2] = h.d[5];
	pInverse[id].d[3] = h.d[6];
	pInverse[id].d[4] = h.d[7];
}

__kernel void profanity_score_benchmark(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
	const size_t id = get_global_id(0);
	__global const uchar * const hash = pInverse[id].d;
	int score = 0;

	profanity_result_update(id, hash, pResult, score, scoreMax, pInverse);
}

__kernel void profanity_score_matching(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
	const size_t id = get_global_id(0);
	__global const uchar * const hash = pInverse[id].d;
	int score = 0;

	for (int i = 0; i < 20; ++i) {
		if (data1[i] > 0 && (hash[i] & data1[i]) == data2[i]) {
			++score;
		}
	}

	profanity_result_update(id, hash, pResult, score, scoreMax, pInverse);
}

__kernel void profanity_score_leading(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
	const size_t id = get_global_id(0);
	int score = 0;

	// 获取地址类型（存储在d[5]中）
	uchar addressType = pInverse[id].d[5] & 0xFF;
	
	// 从pInverse中提取完整的BTC地址字符串
	__private char btc_address[92];
	decode_btc_address_from_pinverse(pInverse, id, btc_address);
	
	// 根据地址类型调整评分逻辑
	// 对于BTC地址，我们需要考虑固有前缀
	int startPos = 0;
	
	switch (addressType) {
		case 0: // Legacy P2PKH: 以"1" 开头
		case 1: // SegWit P2SH: 以"3" 开头
			startPos = 1; // 从第二个字符开始检查
			break;
		case 2: // Native SegWit P2WPKH: 以"bc1q" 开头 
		case 3: // Taproot P2TR: 以"bc1p" 开头
			startPos = 4; // 从第五个字符开始检查
			break;
		default:
			startPos = 0; // 默认从头开始
			break;
	}

	// 从指定位置开始检查完整的BTC地址字符串
	for (int i = startPos; i < 92; ++i) {
		if (btc_address[i] == '\0') break;
		if (btc_address[i] == data1[0]) {
			++score;
		}
		else {
			break;
		}
	}

	// 从hash160数据中提取hash用于profanity_result_update
	__private uchar hash160[20];
	for (int i = 0; i < 5; i++) {
		uint packed = pInverse[id].d[i];
		hash160[i*4] = (packed >> 24) & 0xFF;
		hash160[i*4+1] = (packed >> 16) & 0xFF;
		hash160[i*4+2] = (packed >> 8) & 0xFF;
		hash160[i*4+3] = packed & 0xFF;
	}

	profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}

__kernel void profanity_score_range(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
	const size_t id = get_global_id(0);
	int score = 0;

	// 从pInverse中提取完整的BTC地址字符串
	__private char btc_address[92];
	decode_btc_address_from_pinverse(pInverse, id, btc_address);

	// 检查完整的BTC地址字符串
	for (int i = 0; i < 92; ++i) {
		if (btc_address[i] == '\0') break;
		if (btc_address[i] >= data1[0] && btc_address[i] <= data2[0]) {
			++score;
		}
	}

	// 从hash160数据中提取hash用于profanity_result_update
	__private uchar hash160[20];
	for (int i = 0; i < 5; i++) {
		uint packed = pInverse[id].d[i];
		hash160[i*4] = (packed >> 24) & 0xFF;
		hash160[i*4+1] = (packed >> 16) & 0xFF;
		hash160[i*4+2] = (packed >> 8) & 0xFF;
		hash160[i*4+3] = packed & 0xFF;
	}

	profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}

__kernel void profanity_score_leadingrange(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
	const size_t id = get_global_id(0);
	int score = 0;

	// 从pInverse中提取完整的BTC地址字符串
	__private char btc_address[92];
	decode_btc_address_from_pinverse(pInverse, id, btc_address);

	// 从开头开始检查完整的BTC地址字符串
	for (int i = 0; i < 92; ++i) {
		if (btc_address[i] == '\0') break;
		if (btc_address[i] >= data1[0] && btc_address[i] <= data2[0]) {
			++score;
		}
		else {
			break;
		}
	}

	// 从hash160数据中提取hash用于profanity_result_update
	__private uchar hash160[20];
	for (int i = 0; i < 5; i++) {
		uint packed = pInverse[id].d[i];
		hash160[i*4] = (packed >> 24) & 0xFF;
		hash160[i*4+1] = (packed >> 16) & 0xFF;
		hash160[i*4+2] = (packed >> 8) & 0xFF;
		hash160[i*4+3] = packed & 0xFF;
	}

	profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}

__kernel void profanity_score_mirror(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
	const size_t id = get_global_id(0);
	int score = 0;

	// 从pInverse中提取完整的BTC地址字符串
	__private char btc_address[92];
	decode_btc_address_from_pinverse(pInverse, id, btc_address);

	// 检查镜像对称性
	int addrIndex = 0;
	while (btc_address[addrIndex] != '\0' && addrIndex < 92) {
		addrIndex++;
	}
	
	for (int i = 0; i < 10; ++i) {
		if (i >= addrIndex || (addrIndex - 1 - i) < 0) break;
		
		if (btc_address[i] != btc_address[addrIndex - 1 - i]) {
			break;
		}
		++score;
	}

	// 从hash160数据中提取hash用于profanity_result_update
	__private uchar hash160[20];
	for (int i = 0; i < 5; i++) {
		uint packed = pInverse[id].d[i];
		hash160[i*4] = (packed >> 24) & 0xFF;
		hash160[i*4+1] = (packed >> 16) & 0xFF;
		hash160[i*4+2] = (packed >> 8) & 0xFF;
		hash160[i*4+3] = packed & 0xFF;
	}

	profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}

__kernel void profanity_score_doubles(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
	const size_t id = get_global_id(0);
	int score = 0;

	// 从pInverse中提取完整的BTC地址字符串
	__private char btc_address[92];
	decode_btc_address_from_pinverse(pInverse, id, btc_address);
	
	int addrIndex = 0;
	while (btc_address[addrIndex] != '\0' && addrIndex < 92) {
		addrIndex++;
	}

	// 检查重复字符模式
	for (int i = 0; i < addrIndex; ++i) {
		if (btc_address[i] == '\0') break;
		
		// 检查是否是重复字符（如 "11", "22", "aa" 等）
		if (i + 1 < addrIndex && btc_address[i] == btc_address[i + 1]) {
			++score;
		}
		else {
			break;
		}
	}

	// 从hash160数据中提取hash用于profanity_result_update
	__private uchar hash160[20];
	for (int i = 0; i < 5; i++) {
		uint packed = pInverse[id].d[i];
		hash160[i*4] = (packed >> 24) & 0xFF;
		hash160[i*4+1] = (packed >> 16) & 0xFF;
		hash160[i*4+2] = (packed >> 8) & 0xFF;
		hash160[i*4+3] = packed & 0xFF;
	}

	profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}

// 1. 只匹配地址开始最左边的连续性字符或数字
__kernel void profanity_score_leading_sequential(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
    const size_t id = get_global_id(0);
    int score = 0;
    int current_seq = 0;
    uchar last = 0;
    
    // 从pInverse中提取完整的BTC地址字符串
    __private char btc_address[92];
    decode_btc_address_from_pinverse(pInverse, id, btc_address);
    
    int addrIndex = 0;
    while (btc_address[addrIndex] != '\0' && addrIndex < 92) {
        addrIndex++;
    }
    
    // 严格从最左边第一个字符开始检查
    if (btc_address[0] == '\0') {
        profanity_result_update(id, (__global const uchar*)pInverse[id].d, pResult, 0, scoreMax, pInverse);
        return;
    }
    
    const uchar first_char = btc_address[0];
    last = first_char;
    current_seq = 1;
    
    // 从第二个字符开始检查连续
    for (int i = 1; i < 92 && btc_address[i] != '\0'; ++i) {
        const uchar current = btc_address[i];
        if (current == last + 1 || (current == 0 && last == 15)) {
            current_seq++;
        } else {
            break;
        }
        last = current;
    }
    
    score = current_seq >= data1[0] ? current_seq : 0;
    
    // 从hash160数据中提取hash用于profanity_result_update
    __private uchar hash160[20];
    for (int i = 0; i < 5; i++) {
        uint packed = pInverse[id].d[i];
        hash160[i*4] = (packed >> 24) & 0xFF;
        hash160[i*4+1] = (packed >> 16) & 0xFF;
        hash160[i*4+2] = (packed >> 8) & 0xFF;
        hash160[i*4+3] = packed & 0xFF;
    }
    
    profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}

// 2. 匹配任意位置的连续性字符或数字
__kernel void profanity_score_any_sequential(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
    const size_t id = get_global_id(0);
    int score = 0;
    int current_seq = 1;
    uchar last = 0;
    
    // 从pInverse中提取完整的BTC地址字符串
    __private char btc_address[92];
    decode_btc_address_from_pinverse(pInverse, id, btc_address);
    
    int addrIndex = 0;
    while (btc_address[addrIndex] != '\0' && addrIndex < 92) {
        addrIndex++;
    }
    
    for (int i = 0; i < addrIndex && btc_address[i] != '\0'; ++i) {
        const uchar current = btc_address[i];
        if (i == 0) {
            last = current;
            continue;
        }
        
        if (current == last + 1 || (current == 0 && last == 15)) {
            current_seq++;
            if (current_seq >= data1[0]) {
                score = current_seq;
            }
        } else {
            current_seq = 1;
        }
        last = current;
    }
    
    // 从hash160数据中提取hash用于profanity_result_update
    __private uchar hash160[20];
    for (int i = 0; i < 5; i++) {
        uint packed = pInverse[id].d[i];
        hash160[i*4] = (packed >> 24) & 0xFF;
        hash160[i*4+1] = (packed >> 16) & 0xFF;
        hash160[i*4+2] = (packed >> 8) & 0xFF;
        hash160[i*4+3] = packed & 0xFF;
    }
    
    profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}

// 3. 只匹配地址结束位置(最右边)的连续性字符或数字
__kernel void profanity_score_ending_sequential(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
    const size_t id = get_global_id(0);
    int score = 0;
    int current_seq = 0;
    
    // 从pInverse中提取完整的BTC地址字符串
    __private char btc_address[92];
    decode_btc_address_from_pinverse(pInverse, id, btc_address);
    
    int addrIndex = 0;
    while (btc_address[addrIndex] != '\0' && addrIndex < 92) {
        addrIndex++;
    }
    
    // 严格从最右边最后一个字符开始检查
    if (addrIndex == 0) {
        profanity_result_update(id, (__global const uchar*)pInverse[id].d, pResult, 0, scoreMax, pInverse);
        return;
    }
    
    const uchar last_char = btc_address[addrIndex - 1];
    uchar last = last_char;
    current_seq = 1;
    
    // 从倒数第二个字符开始向前检查连续性
    for (int i = addrIndex - 2; i >= 0 && i >= addrIndex - data1[0]; --i) {
        const uchar current = btc_address[i];
        if (current + 1 == last || (current == 15 && last == 0)) {
            current_seq++;
        } else {
            break;
        }
        last = current;
    }
    
    score = current_seq >= data1[0] ? current_seq : 0;
    
    // 从hash160数据中提取hash用于profanity_result_update
    __private uchar hash160[20];
    for (int i = 0; i < 5; i++) {
        uint packed = pInverse[id].d[i];
        hash160[i*4] = (packed >> 24) & 0xFF;
        hash160[i*4+1] = (packed >> 16) & 0xFF;
        hash160[i*4+2] = (packed >> 8) & 0xFF;
        hash160[i*4+3] = packed & 0xFF;
    }
    
    profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}

// 4. 只匹配地址开头(最左边)的指定字符
__kernel void profanity_score_leading_specific(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
    const size_t id = get_global_id(0);
    int score = 0;
    int pattern_len = 0;
    
    // 获取地址类型（存储在d[5]中）
    uchar addressType = pInverse[id].d[5] & 0xFF;
    
    // 从pInverse中提取完整的BTC地址字符串
    __private char btc_address[92];
    decode_btc_address_from_pinverse(pInverse, id, btc_address);
    
    int addrIndex = 0;
    while (btc_address[addrIndex] != '\0' && addrIndex < 92) {
        addrIndex++;
    }
    
    // 根据地址类型调整起始位置
    // 对于BTC地址，我们需要考虑固有前缀
    int startPos = 0;
    
    switch (addressType) {
        case 0: // Legacy P2PKH: 以"1" 开头
        case 1: // SegWit P2SH: 以"3" 开头
            startPos = 1; // 从第二个字符开始检查
            break;
        case 2: // Native SegWit P2WPKH: 以"bc1q" 开头 
        case 3: // Taproot P2TR: 以"bc1p" 开头
            startPos = 4; // 从第五个字符开始检查
            break;
        default:
            startPos = 0; // 默认从头开始
            break;
    }
    
    // 计算模式长度
    while (data1[pattern_len] != 0 && pattern_len < 20) pattern_len++;
    
    // 从指定位置开始匹配完整的BTC地址字符串
    for (int i = 0; i < pattern_len; ++i) {
        if (startPos + i >= 92 || btc_address[startPos + i] == '\0') {
            return; // 超出地址长度
        }
        if (btc_address[startPos + i] != data1[i]) {
            return; // 如果不匹配,直接返回
        }
        score++;
    }
    
    // 从hash160数据中提取hash用于profanity_result_update
    __private uchar hash160[20];
    for (int i = 0; i < 5; i++) {
        uint packed = pInverse[id].d[i];
        hash160[i*4] = (packed >> 24) & 0xFF;
        hash160[i*4+1] = (packed >> 16) & 0xFF;
        hash160[i*4+2] = (packed >> 8) & 0xFF;
        hash160[i*4+3] = packed & 0xFF;
    }
    
    profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}

// 5. 匹配地址任意位置的指定字符
__kernel void profanity_score_any_specific(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
    const size_t id = get_global_id(0);
    int score = 0;
    int pattern_len = 0;
    
    // 获取地址类型（存储在d[5]中）
    uchar addressType = pInverse[id].d[5] & 0xFF;
    
    // 从pInverse中提取完整的BTC地址字符串
    __private char btc_address[92];
    decode_btc_address_from_pinverse(pInverse, id, btc_address);
    
    int addrIndex = 0;
    while (btc_address[addrIndex] != '\0' && addrIndex < 92) {
        addrIndex++;
    }
    
    // 根据地址类型调整搜索范围
    // 对于BTC地址，我们需要考虑固有前缀
    int searchStart = 0;
    
    switch (addressType) {
        case 0: // Legacy P2PKH: 以"1" 开头
        case 1: // SegWit P2SH: 以"3" 开头
            searchStart = 1; // 从第二个字符开始搜索
            break;
        case 2: // Native SegWit P2WPKH: 以"bc1q" 开头 
        case 3: // Taproot P2TR: 以"bc1p" 开头
            searchStart = 4; // 从第五个字符开始搜索
            break;
        default:
            searchStart = 0; // 默认从头开始
            break;
    }
    
    // 计算模式长度
    while (data1[pattern_len] != 0 && pattern_len < 20) pattern_len++;
    
    // 在指定范围内查找匹配完整的BTC地址字符串
    for (int i = searchStart; i < 92 - pattern_len; ++i) {
        if (btc_address[i] == '\0') break; // 超出地址长度
        
        int matched = 0;
        for (int j = 0; j < pattern_len; ++j) {
            if (i + j >= 92 || btc_address[i + j] == '\0') {
                break;
            }
            if (btc_address[i + j] != data1[j]) {
                break;
            }
            matched++;
        }
        if (matched == pattern_len) {
            score = matched;
            break;
        }
    }
    
    // 从hash160数据中提取hash用于profanity_result_update
    __private uchar hash160[20];
    for (int i = 0; i < 5; i++) {
        uint packed = pInverse[id].d[i];
        hash160[i*4] = (packed >> 24) & 0xFF;
        hash160[i*4+1] = (packed >> 16) & 0xFF;
        hash160[i*4+2] = (packed >> 8) & 0xFF;
        hash160[i*4+3] = packed & 0xFF;
    }
    
    profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}

// 6. 只匹配地址结束(最右边)的指定字符
__kernel void profanity_score_ending_specific(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
    const size_t id = get_global_id(0);
    int score = 0;
    int pattern_len = 0;
    
    // 从pInverse中提取完整的BTC地址字符串
    __private char btc_address[92];
    decode_btc_address_from_pinverse(pInverse, id, btc_address);
    
    int addrIndex = 0;
    while (btc_address[addrIndex] != '\0' && addrIndex < 92) {
        addrIndex++;
    }
    
    // 计算模式长度
    while (data1[pattern_len] != 0 && pattern_len < 20) pattern_len++;
    
    // 严格从最右边开始匹配（结尾匹配不受地址类型影响）
    for (int i = 0; i < pattern_len; ++i) {
        if (addrIndex - 1 - i < 0) {
            return; // 超出地址长度
        }
        if (btc_address[addrIndex - 1 - i] != data1[pattern_len - 1 - i]) {
            return; // 如果结尾不匹配直接返回
        }
        score++;
    }
    
    // 从hash160数据中提取hash用于profanity_result_update
    __private uchar hash160[20];
    for (int i = 0; i < 5; i++) {
        uint packed = pInverse[id].d[i];
        hash160[i*4] = (packed >> 24) & 0xFF;
        hash160[i*4+1] = (packed >> 16) & 0xFF;
        hash160[i*4+2] = (packed >> 8) & 0xFF;
        hash160[i*4+3] = packed & 0xFF;
    }
    
    profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}

// 7. 只匹配地址开头(最左边)的连续出现相同的字符或数字
__kernel void profanity_score_leading_same(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
    const size_t id = get_global_id(0);
    int score = 0;
    int current_seq = 1;
    
    // 从pInverse中提取完整的BTC地址字符串
    __private char btc_address[92];
    decode_btc_address_from_pinverse(pInverse, id, btc_address);
    
    int addrIndex = 0;
    while (btc_address[addrIndex] != '\0' && addrIndex < 92) {
        addrIndex++;
    }
    
    // 严格从最左边第一个字符开始
    if (addrIndex == 0) {
        profanity_result_update(id, (__global const uchar*)pInverse[id].d, pResult, 0, scoreMax, pInverse);
        return;
    }
    
    const uchar first_char = btc_address[0];
    
    // 检查后续字符是否与第一个字符相同
    for (int i = 1; i < addrIndex && btc_address[i] != '\0'; ++i) {
        if (btc_address[i] == first_char) {
            current_seq++;
        } else {
            break;
        }
    }
    
    score = current_seq >= data1[0] ? current_seq : 0;
    
    // 从hash160数据中提取hash用于profanity_result_update
    __private uchar hash160[20];
    for (int i = 0; i < 5; i++) {
        uint packed = pInverse[id].d[i];
        hash160[i*4] = (packed >> 24) & 0xFF;
        hash160[i*4+1] = (packed >> 16) & 0xFF;
        hash160[i*4+2] = (packed >> 8) & 0xFF;
        hash160[i*4+3] = packed & 0xFF;
    }
    
    profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}

// 8. 匹配任意位置的连续出现相同的字符或数字
__kernel void profanity_score_any_same(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data1, __constant const uchar * const data2, const uchar scoreMax) {
    const size_t id = get_global_id(0);
    int score = 0;
    int current_seq = 1;
    uchar last = 0;
    
    // 从pInverse中提取完整的BTC地址字符串
    __private char btc_address[92];
    decode_btc_address_from_pinverse(pInverse, id, btc_address);
    
    int addrIndex = 0;
    while (btc_address[addrIndex] != '\0' && addrIndex < 92) {
        addrIndex++;
    }
    
    for (int i = 0; i < addrIndex && btc_address[i] != '\0'; ++i) {
        const uchar current = btc_address[i];
        if (i == 0) {
            last = current;
            continue;
        }
        
        if (current == last) {
            current_seq++;
            if (current_seq >= data1[0]) {
                score = current_seq;
            }
        } else {
            current_seq = 1;
        }
        last = current;
    }
    
    // 从hash160数据中提取hash用于profanity_result_update
    __private uchar hash160[20];
    for (int i = 0; i < 5; i++) {
        uint packed = pInverse[id].d[i];
        hash160[i*4] = (packed >> 24) & 0xFF;
        hash160[i*4+1] = (packed >> 16) & 0xFF;
        hash160[i*4+2] = (packed >> 8) & 0xFF;
        hash160[i*4+3] = packed & 0xFF;
    }
    
    profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}

// 9. 只匹配地址结束位置(最右边)的连续出现相同的字符或数字
__kernel void profanity_score_ending_same(__global mp_number * const pInverse, __global result * const pResult, __constant const uchar * const data2, const uchar scoreMax) {
    const size_t id = get_global_id(0);
    int score = 0;
    int current_seq = 1;
    
    // 从pInverse中提取完整的BTC地址字符串
    __private char btc_address[92];
    decode_btc_address_from_pinverse(pInverse, id, btc_address);
    
    int addrIndex = 0;
    while (btc_address[addrIndex] != '\0' && addrIndex < 92) {
        addrIndex++;
    }
    
    // 严格从最右边最后一个字符开始
    if (addrIndex == 0) {
        profanity_result_update(id, (__global const uchar*)pInverse[id].d, pResult, 0, scoreMax, pInverse);
        return;
    }
    
    const uchar last_char = btc_address[addrIndex - 1];
    
    // 从倒数第二个字符开始向前检查
    for (int i = addrIndex - 2; i >= 0 && i >= addrIndex - data1[0]; --i) {
        if (btc_address[i] == last_char) {
            current_seq++;
        } else {
            break;
        }
    }
    
    score = current_seq >= data1[0] ? current_seq : 0;
    
    // 从hash160数据中提取hash用于profanity_result_update
    __private uchar hash160[20];
    for (int i = 0; i < 5; i++) {
        uint packed = pInverse[id].d[i];
        hash160[i*4] = (packed >> 24) & 0xFF;
        hash160[i*4+1] = (packed >> 16) & 0xFF;
        hash160[i*4+2] = (packed >> 8) & 0xFF;
        hash160[i*4+3] = packed & 0xFF;
    }
    
    profanity_result_update(id, hash160, pResult, score, scoreMax, pInverse);
}




