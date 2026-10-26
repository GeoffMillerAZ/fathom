package reliabilitystats

// #Verdict is the predicted-null outcome; equivalence takes precedence.
#Verdict: "equivalent" | "harm" | "inconclusive"

// #TOSTResult also reports the verdict and fixed one-sided 95% upper bound.
#TOSTResult: {
	unexpected_effect: bool
	if unexpected_effect {verdict: "inconclusive"}
	verdict:  #Verdict
	upper_95: number
	if verdict == "equivalent" {equivalent: true}
	if verdict != "equivalent" {equivalent: false}
	if verdict == "harm" {upper_95: <0}
}
