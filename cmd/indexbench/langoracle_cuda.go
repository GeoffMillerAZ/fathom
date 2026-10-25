package main

// langoracle_cuda.go — item cuda (LB-Q-cuda L4): the cuda dialect resolves its #include statements as ingest
// resolves them (fusion.ResolveFileEdges under the dialect's registry key), so with no SCIP index its file edges are
// scored against the neutral oracle file's pairs, as objective-c's are. The measured set is the .cu and .cuh files;
// a .h a cuda file includes belongs to the C or C++ router (langoracle.go, fileresolve.go headerLangs).
func init() {
	fileResolveLang["cuda"] = "cuda"
}
