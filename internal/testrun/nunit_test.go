package testrun

import (
	"strings"
	"testing"
)

const nativeNUnitFixture = `<?xml version="1.0" encoding="utf-8"?>
<test-run id="2" testcasecount="4" result="Failed" total="4" passed="1" failed="1" inconclusive="1" skipped="1" asserts="0" engine-version="3.5.0.0" start-time="2026-10-19 12:00:01Z" end-time="2026-10-19 12:00:03Z" duration="2">
 <test-suite type="Assembly" id="10" name="Workshop.Tests.dll" fullname="/local/Workshop.Tests.dll" testcasecount="4" result="Failed">
  <test-suite type="ParameterizedMethod" id="11" name="Fits" fullname="Workshop.WorkTests.Fits" testcasecount="2" result="Failed">
   <test-case id="100" name="Fits(1)" fullname="Workshop.WorkTests.Fits(1)" classname="Workshop.WorkTests" methodname="Fits" result="Passed" duration="0.1"/>
   <test-case id="101" name="Fits(2)" fullname="Workshop.WorkTests.Fits(2)" classname="Workshop.WorkTests" methodname="Fits" result="Failed" duration="0.2"><failure><message>expected a fit</message><stack-trace>at Workshop.WorkTests.Fits () [0x00000] in /local/Assets/WorkTests.cs:42</stack-trace></failure></test-case>
  </test-suite>
  <test-case id="102" name="NeedsDevice" fullname="Workshop.WorkTests.NeedsDevice" classname="Workshop.WorkTests" methodname="NeedsDevice" result="Skipped" label="Explicit" duration="0"><reason><message>not selected</message></reason></test-case>
  <test-case id="103" name="NeedsClock" fullname="Workshop.WorkTests.NeedsClock" classname="Workshop.WorkTests" methodname="NeedsClock" result="Inconclusive" duration="0"><reason><message>clock absent</message></reason></test-case>
 </test-suite>
</test-run>`

func TestUnityNUnitHookRecognizesNativeXMLWithoutJUnit(t *testing.T) {
	r := ParseTestRun(nativeNUnitFixture)
	if r.Runner != Runner("unity-nunit") || !r.Complete || !r.Failed() {
		t.Fatalf("native NUnit must be a complete failed native run, not JUnit or unknown: %+v", r)
	}
	if len(r.Failures) != 1 || r.Failures[0].ID() != "Workshop.WorkTests.Fits(2)" {
		t.Fatalf("parameterized failure identity lost: %+v", r.Failures)
	}
	if r.Summary == nil || r.Summary.Tests != 4 || r.Summary.Failures != 1 || r.Summary.Skipped != 2 {
		t.Fatalf("native counts must retain nonpassing skip/inconclusive: %+v", r.Summary)
	}
}

func TestUnityNUnitPreservesNativeOutcomeAndParameterizedIdentity(t *testing.T) {
	run, err := ParseUnityNUnitXML([]byte(nativeNUnitFixture))
	if err != nil {
		t.Fatal(err)
	}
	if run.EngineVersion != "3.5.0.0" || len(run.Cases) != 4 || run.Inconclusive != 1 {
		t.Fatalf("metadata: %+v", run)
	}
	var failed *NUnitCase
	for i := range run.Cases {
		if run.Cases[i].ID == "101" {
			failed = &run.Cases[i]
		}
	}
	if failed == nil || failed.Assembly != "Workshop.Tests" || failed.FullName != "Workshop.WorkTests.Fits(2)" || failed.MethodName != "Fits" || failed.Duration != "0.2" || failed.Message != "expected a fit" || !strings.Contains(failed.StackTrace, "WorkTests.cs:42") {
		t.Fatalf("native case lost information: %+v", failed)
	}
	for _, g := range run.Cohorts {
		if g.MethodName == "Fits" && (!g.Complete || g.Observed != 2 || g.Expected != 2) {
			t.Fatalf("cohort: %+v", g)
		}
	}
}

func TestUnityNUnitRefusesMalformedContradictoryAndDuplicateEvidence(t *testing.T) {
	cases := map[string]string{
		"nested case smuggling": strings.Replace(nativeNUnitFixture, `duration="0.1"/>`, `duration="0.1"><test-case id="hidden"/></test-case>`, 1),
		"missing duration":      strings.Replace(nativeNUnitFixture, ` duration="0.2"`, "", 1),

		"truncated":                   strings.TrimSuffix(nativeNUnitFixture, "</test-run>"),
		"duplicate id":                strings.Replace(nativeNUnitFixture, `id="101"`, `id="100"`, 1),
		"duplicate identity":          strings.Replace(nativeNUnitFixture, `fullname="Workshop.WorkTests.Fits(2)"`, `fullname="Workshop.WorkTests.Fits(1)"`, 1),
		"unsupported result":          strings.Replace(nativeNUnitFixture, `result="Failed" duration="0.2"`, `result="Maybe" duration="0.2"`, 1),
		"DTD":                         `<!DOCTYPE test-run [<!ENTITY x SYSTEM "file:///secret">]>` + nativeNUnitFixture,
		"invalid UTF8":                nativeNUnitFixture + string([]byte{0xff}),
		"trailing root":               nativeNUnitFixture + `<extra/>`,
		"contradictory total":         strings.Replace(nativeNUnitFixture, `total="4"`, `total="3"`, 1),
		"negative counter":            strings.Replace(nativeNUnitFixture, `passed="1"`, `passed="-1"`, 1),
		"NaN duration":                strings.Replace(nativeNUnitFixture, `duration="0.2"`, `duration="NaN"`, 1),
		"missing method":              strings.Replace(nativeNUnitFixture, `methodname="Fits"`, `methodname=""`, 1),
		"missing assembly":            strings.Replace(nativeNUnitFixture, `type="Assembly"`, `type="TestFixture"`, 1),
		"more variants than declared": strings.Replace(nativeNUnitFixture, `testcasecount="2"`, `testcasecount="1"`, 1),
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseUnityNUnitXML([]byte(text)); err == nil {
				t.Fatal("malformed native evidence accepted")
			}
		})
	}
}

func TestUnityNUnitHookCannotTurnInvalidOrSkippedXMLIntoPass(t *testing.T) {
	zero := 0
	invalid := strings.Replace(nativeNUnitFixture, `total="4"`, `total="3"`, 1)
	if got := Analyze(invalid, Evidence{ExitCode: &zero}).Verdict; got != Unknown {
		t.Fatalf("invalid native XML + exit0 = %s", got)
	}
	skipped := strings.NewReplacer(`result="Failed"`, `result="Skipped"`, `failed="1"`, `failed="0"`, `skipped="1"`, `skipped="2"`).Replace(nativeNUnitFixture)
	if got := Analyze(skipped, Evidence{ExitCode: &zero}).Verdict; got != Unknown {
		t.Fatalf("native nonpassing outcomes + exit0 = %s", got)
	}
}

func TestUnityNUnitDispatchDoesNotShadowJUnitWithNativeText(t *testing.T) {
	text := `<?xml version="1.0"?><testsuite tests="1" failures="1"><testcase classname="Other" name="TestA"><failure><![CDATA[mentions <test-run> as data]]></failure></testcase></testsuite>`
	if got := ParseTestRun(text); got.Runner != RunnerJUnit || !got.Failed() {
		t.Fatalf("JUnit dispatch changed: %+v", got)
	}
}

func TestUnityNUnitVariantCohortNeverSelectsAnArbitraryLeaf(t *testing.T) {
	run, err := ParseUnityNUnitXML([]byte(nativeNUnitFixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range run.Cohorts {
		if c.MethodName == "Fits" && c.ObservedVerdict != Fail {
			t.Fatalf("failed variant hidden: %+v", c)
		}
	}
	passed := strings.NewReplacer(`result="Failed"`, `result="Passed"`, `failed="1"`, `failed="0"`, `passed="1"`, `passed="2"`).Replace(nativeNUnitFixture)
	// The run still has nonpassing other methods; only Fits is a full pass.
	passed = strings.Replace(passed, `id="2" testcasecount="4" result="Passed"`, `id="2" testcasecount="4" result="Inconclusive"`, 1)
	run, err = ParseUnityNUnitXML([]byte(passed))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range run.Cohorts {
		if c.MethodName == "Fits" && c.ObservedVerdict != Pass {
			t.Fatalf("complete all-passed variants: %+v", c)
		}
	}
	// A filter may return one variant while the native method declares two.
	missing := strings.Replace(passed, `testcasecount="2"`, `testcasecount="3"`, 1)
	run, err = ParseUnityNUnitXML([]byte(missing))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range run.Cohorts {
		if c.MethodName == "Fits" && (c.Complete || c.ObservedVerdict != Unknown) {
			t.Fatalf("missing variant became a pass: %+v", c)
		}
	}
}

func TestUnityNUnitHookDoesNotPromoteMissingParameterizedVariant(t *testing.T) {
	raw := `<test-run id="r" result="Passed" testcasecount="2" total="1" passed="1" failed="0" skipped="0" inconclusive="0" duration="1" start-time="2026-10-19 12:00:00Z" end-time="2026-10-19 12:00:01Z"><test-suite type="Assembly" name="Workshop.Tests.dll"><test-suite type="ParameterizedMethod" id="p" testcasecount="2"><test-case id="c" name="Fits(1)" fullname="Workshop.BowlTests.Fits(1)" classname="Workshop.BowlTests" methodname="Fits" result="Passed" duration="1"/></test-suite></test-suite></test-run>`
	zero := 0
	if got := Analyze(raw, Evidence{ExitCode: &zero}).Verdict; got != Unknown {
		t.Fatalf("missing variant promoted to %s", got)
	}
}

func TestUnityNUnitPreservesUnityChildFailureState(t *testing.T) {
	raw := strings.Replace(nativeNUnitFixture, `result="Failed"`, `result="Failed(Child)"`, 1)
	run, err := ParseUnityNUnitXML([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if run.Result != "Failed(Child)" || run.Failed != 1 {
		t.Fatalf("raw child failure lost %+v", run)
	}
	if got := Analyze(raw, Evidence{}).Verdict; got != Fail {
		t.Fatalf("child failure observed %s", got)
	}
	unknown := strings.Replace(raw, `Failed(Child)`, `Failed(Unsupported)`, 1)
	if _, err := ParseUnityNUnitXML([]byte(unknown)); err == nil {
		t.Fatal("arbitrary compound outcome accepted")
	}
}
