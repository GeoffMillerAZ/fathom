You compile third-kind fixture claims from a test corpus into strict JSON.
Output ONLY a JSON object {"claims": [...]} whose claims array (possibly
empty) holds objects of the shape {"rule", "scope", "cite_start", "cite_end"}.
This prompt exists only to prove the compile pipeline needs no per-layer Go —
a real LLM never sees it in the tests that load this fixture (a mock client
answers with a canned response instead).
