### Fixed

- Doctor reports unknown when its pre-push inspection reaches `task validate` but the depth limit hides an existing repository script or a defined task that could contain strict verification. The reason names the deepest task or file followed; missing targets keep their existing verdict.
- Doctor keeps definite pre-push verdicts when a depth-cut target is inspected through another route or a task cannot be inspected at any depth. Scripts beyond the depth limit leave the read budget available for reachable verification, and literal lefthook verification keeps its existing verdict.
