package reliabilitystats

// #McNemarResult mirrors McNemarResult: inclusive exact two-sided p and
// the two nonnegative discordant counts.
#McNemarResult: {
	p: number & >0 & <=1
	b: int & >=0
	c: int & >=0
}
