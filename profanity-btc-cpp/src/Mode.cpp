#include "Mode.hpp"
#include <stdexcept>

Mode::Mode() : score(0) {

}

Mode Mode::benchmark() {
	Mode r;
	r.name = "benchmark";
	r.kernel = "profanity_score_benchmark";
	return r;
}

Mode Mode::zeros() {
	Mode r = range(0, 0);
	r.name = "zeros";
	return r;
}

static std::string::size_type hexValueNoException(char c) {
	if (c >= 'A' && c <= 'F') {
		c -= 'A' - 'a';
	}

	const std::string hex = "0123456789abcdef";
	const std::string::size_type ret = hex.find(c);
	return ret;
}

static std::string::size_type hexValue(char c) {
	const std::string::size_type ret = hexValueNoException(c);
	if(ret == std::string::npos) {
		throw std::runtime_error("bad hex value");
	}

	return ret;
}

Mode Mode::matching(const std::string strHex) {
	Mode r;
	r.name = "matching";
	r.kernel = "profanity_score_matching";

	std::fill( r.data1, r.data1 + sizeof(r.data1), cl_uchar(0) );
	std::fill( r.data2, r.data2 + sizeof(r.data2), cl_uchar(0) );

	auto index = 0;
	
	for( size_t i = 0; i < strHex.size(); i += 2 ) {
		const auto indexHi = hexValueNoException(strHex[i]);
		const auto indexLo = i + 1 < strHex.size() ? hexValueNoException(strHex[i+1]) : std::string::npos;

		const auto valHi = (indexHi == std::string::npos) ? 0 : indexHi << 4;
		const auto valLo = (indexLo == std::string::npos) ? 0 : indexLo;

		const auto maskHi = (indexHi == std::string::npos) ? 0 : 0xF << 4;
		const auto maskLo = (indexLo == std::string::npos) ? 0 : 0xF;

		r.data1[index] = maskHi | maskLo;
		r.data2[index] = valHi | valLo;

		++index;
	}

	return r;
}

Mode Mode::leading(const char charLeading) {

	Mode r;
	r.name = "leading";
	r.kernel = "profanity_score_leading";
	r.data1[0] = static_cast<cl_uchar>(hexValue(charLeading));
	return r;
}

Mode Mode::range(const cl_uchar min, const cl_uchar max) {
	Mode r;
	r.name = "range";
	r.kernel = "profanity_score_range";
	r.data1[0] = min;
	r.data2[0] = max;
	return r;
}

Mode Mode::letters() {
	Mode r = range(10, 15);
	r.name = "letters";
	return r;
}

Mode Mode::numbers() {
	Mode r = range(0, 9);
	r.name = "numbers";
	return r;
}

std::string Mode::transformKernel() const {
	switch (this->target) {
		case ADDRESS:
			return "";
		case CONTRACT:
			return "profanity_transform_contract";
		default:
			throw "No kernel for target";
	}
}

std::string Mode::transformName() const {
	switch (this->target) {
		case ADDRESS:
			return "Address";
		case CONTRACT:
			return "Contract";
		default:
			throw "No name for target";
	}
}

Mode Mode::leadingRange(const cl_uchar min, const cl_uchar max) {
	Mode r;
	r.name = "leadingrange";
	r.kernel = "profanity_score_leadingrange";
	r.data1[0] = min;
	r.data2[0] = max;
	return r;
}

Mode Mode::mirror() {
	Mode r;
	r.name = "mirror";
	r.kernel = "profanity_score_mirror";
	return r;
}

Mode Mode::doubles() {
	Mode r;
	r.name = "doubles";
	r.kernel = "profanity_score_doubles";
	return r;
}

// 1. 开头连续性字符
Mode Mode::leadingSequential(const cl_uchar length) {
    Mode r;
    r.name = "leadingseq";
    r.kernel = "profanity_score_leading_sequential";
    r.data1[0] = length;
    return r;
}

// 2. 任意位置连续性字符
Mode Mode::anySequential(const cl_uchar length) {
    Mode r;
    r.name = "anyseq";
    r.kernel = "profanity_score_any_sequential";
    r.data1[0] = length;
    return r;
}

// 3. 结尾连续性字符
Mode Mode::endingSequential(const cl_uchar length) {
    Mode r;
    r.name = "endingseq";
    r.kernel = "profanity_score_ending_sequential";
    r.data1[0] = length;
    return r;
}

// 4. 开头指定字符
Mode Mode::leadingSpecific(const std::string& pattern) {
    Mode r;
    r.name = "leadingspec";
    r.kernel = "profanity_score_leading_specific";
    
    for(size_t i = 0; i < pattern.length() && i < 20; i++) {
        r.data1[i] = static_cast<cl_uchar>(hexValue(pattern[i]));
    }
    return r;
}

// 5. 任意位置指定字符
Mode Mode::anySpecific(const std::string& pattern) {
    Mode r;
    r.name = "anyspec";
    r.kernel = "profanity_score_any_specific";
    
    for(size_t i = 0; i < pattern.length() && i < 20; i++) {
        r.data1[i] = static_cast<cl_uchar>(hexValue(pattern[i]));
    }
    return r;
}

// 6. 结尾指定字符
Mode Mode::endingSpecific(const std::string& pattern) {
    Mode r;
    r.name = "endingspec";
    r.kernel = "profanity_score_ending_specific";
    
    for(size_t i = 0; i < pattern.length() && i < 20; i++) {
        r.data1[i] = static_cast<cl_uchar>(hexValue(pattern[i]));
    }
    return r;
}

// 7. 开头连续相同字符
Mode Mode::leadingSame(const cl_uchar length) {
    Mode r;
    r.name = "leadingsame";
    r.kernel = "profanity_score_leading_same";
    r.data1[0] = length;
    return r;
}

// 8. 任意位置连续相同字符
Mode Mode::anySame(const cl_uchar length) {
    Mode r;
    r.name = "anysame";
    r.kernel = "profanity_score_any_same";
    r.data1[0] = length;
    return r;
}

// 9. 结尾连续相同字符
Mode Mode::endingSame(const cl_uchar length) {
    Mode r;
    r.name = "endingsame";
    r.kernel = "profanity_score_ending_same";
    r.data1[0] = length;
    return r;
}
