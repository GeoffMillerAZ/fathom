package main

// skills_shipped_renders.go — FA-24 (docs/plans/fleet-adoption-fixes.md
// Bug 5, O657): the sha256 of every render of a shipped skill that fathom
// released, from git history up to 2026-10-09: each historical
// dist/agents/skills/<name>/SKILL.md (the .claude/skills render, verbatim;
// its .agents/skills rendering, agentsVariant, is byte-identical for every
// version) and codex-prompt.md (the ~/.codex/prompts twin).
//
// A skill file under fathom's marker whose bytes hash to one of these is
// fathom's own older render: stale, rewritten by `fleet reconcile --apply`,
// never drift, and the only content `--rebaseline` re-records besides this
// binary's render. Before FA-23 two writers recorded no baseline for what
// they wrote (`fathom init --skills`, and reconcile's apply for the skill
// id), so an adopter's skills sit at renders the per-checkout ledger never
// saw; this list is how a later binary still knows them.
//
// TestShippedSkillRenderDigestsCoverThisBinary keeps it complete: a skill
// edit appends the new render's digest (the failure prints the lines). The
// list was generated with, per name and file, `git log --format=%h -- <path>`
// and `git show <h>:<path> | shasum -a 256`.

// shippedSkillRenderDigests returns the list as a fresh set: each caller
// owns its copy, so no map is shared across the daemon's requests.
func shippedSkillRenderDigests() map[string]bool {
	m := make(map[string]bool, len(shippedSkillRenderDigestList))
	for _, d := range shippedSkillRenderDigestList {
		m[d] = true
	}
	return m
}

var shippedSkillRenderDigestList = []string{
	"sha256:0076bbd2a1e36a692ec148f0e29ec3b944c81f9283e3c2f6a8ada5f02cbab869", // fathom-onboard claude 2026-09-02 9c1d9b16
	"sha256:0295bb2b4e4d5599225afaf521636da6a2cd1dd3e8e52a2167801d7655a1a553", // fathom-impact claude 2026-08-13 17158885
	"sha256:0762cd51889a97ce5f94f7ca4c383939a5db95264e1762e58f08e7c6d9234440", // fathom-plan claude 2026-09-28 ccd97a0b
	"sha256:123f96857c4bce2c317fafb1eb08af4af9ca42bc0e5f429fab361d0f588fad78", // fathom-onboard codex-prompts 2026-10-10 h4-cgofree
	"sha256:15a087f2f46263066bf306d178f61b5af8f0d6ee125c68a6c65e1a8d78376f51", // fathom-custodian claude 2026-08-25 2d070017
	"sha256:16bdf3905c9a829e9f41606e84ed27f245ef0379c4f37af0397311d25c29f88e", // fathom-plan codex-prompts 2026-07-23 3e4f8ea2
	"sha256:172103f097b66cc22eb2387764faf7d42f39986f3248bbf2bf4e406c34cd5bc4", // fathom-doctor codex-prompts 2026-07-23 7a59507b
	"sha256:174e8cdfc7baa51d93c9cbf92970ee522447701b13965a34e04e11f06d78d0b4", // fathom-feedback claude 2026-07-23 370300ff
	"sha256:1dd27468e8652182cbec65c1f26f154d6726bd935bdc982dbd0a6b743d4f188c", // fathom-onboard codex-prompts 2026-07-29 c8200b05
	"sha256:24da5909d7d51f85801a31ec13ae7556ae4b8dc6ca46cc0a1d3b1f2929eba1af", // fathom-verify claude 2026-09-02 9c1d9b16
	"sha256:26d96924548467d184783aa60b507d3cfc14d7bd80d32f98e7f80b08f7b88e66", // fathom-custodian claude 2026-10-03 6a7c8087
	"sha256:2756b791d65c2de5ddd811eff95d3e1c2dad52aaf942bf0459ad8e221571b043", // fathom-impact codex-prompts 2026-07-23 7a59507b
	"sha256:289fd6d846d1960206d0fddfaad2bd73b6c7659fbd414fe10a2c8c26cc043208", // fathom-doctor claude 2026-09-02 af1be1d6
	"sha256:299c9f2fad2532b3649a193ea322d7db48e1bb0b1dcecf7f113c1503f9806d5f", // fathom-feedback codex-prompts 2026-07-23 b5176c15
	"sha256:2a451877a101424691f78f2ba30f51ed8c2900b844a94069bac596c091c5c4d3", // fathom-custodian codex-prompts 2026-09-25 fb5ccfbf
	"sha256:304c58d62c94cfc669d33af6e10698a458c3a22147a0dc42d6bcd2dec3d80041", // fathom-activate claude 2026-09-02 9c1d9b16
	"sha256:3108b49010a113e0132b4458ab2f39b34135a70e19417385df32559ddae7db4b", // fathom-activate claude 2026-08-13 17158885
	"sha256:327cefe745ac0d92e3a06420f919eee68b180c9c8b5e9bc11db639d0bf660bfc", // fathom-onboard claude 2026-09-18 023e9d72
	"sha256:34364c80c1908b53431ed84e1fcd53832f120c181d976642c6ad45d12a9d3a9d", // fathom-custodian codex-prompts 2026-09-02 9c1d9b16
	"sha256:3b001f75443242268a3d1b037460732609c80024f68562075731c8270d87b746", // fathom-onboard codex-prompts 2026-09-12 f7227af0
	"sha256:3ce83a3f1587825213ccf9a32557780fac15ded4b22dfa71a024b4d025413400", // fathom-verify claude 2026-09-02 af1be1d6
	"sha256:3d5f2ccc98f087279719f638e8a775671feecc7d27ebb1fd45c1d20fd33ceb5a", // fathom-activate claude 2026-09-29 110eb2ad
	"sha256:3dffbc4e9c283dbdc69182663e7f3c5f3c5b37700566b911cefe2afbabd9ae0e", // fathom-doctor claude 2026-09-12 7bd3f13b
	"sha256:3ec9ff0f14bb7962323a9b010b0042bce0e54df860fa69fc82b14af03c0ef3b3", // fathom-onboard claude 2026-09-13 d3a5af69
	"sha256:43cb50b6e0a6228c5ea066b29c7c44dad45ccaa61cb48ef2c42052ece7c143bd", // fathom-rewire claude 2026-08-07 1030ad40
	"sha256:477fb013a1fe265bf6c652b20a7d969a71ae48c490b41881a24d6f85be1e7f1c", // fathom-doctor claude 2026-09-13 d3a5af69
	"sha256:4784309ef839d091049a75d2a78ea31a8aea4eebb7b7faaee8682f185398e087", // fathom-activate claude 2026-09-13 088d0458
	"sha256:4aad5786cfbac6e6533f23ad0824ba256261cac3fdc4d5fa1e71cff629ed4f30", // fathom-impact codex-prompts 2026-07-28 8483a7b8
	"sha256:4b1227156d73c08a9a729e943f4d5cdcd728589b4b70658578c18f011d3d207a", // fathom-rewire claude 2026-09-12 f7227af0
	"sha256:4c2282ac84d02fa524dd7addf59f9b849e89b4323b5cb34058174d9401f43bd2", // fathom-rewire codex-prompts 2026-09-29 110eb2ad
	"sha256:4e534a6594f90ee7697d2da69f680337d69f5be3aaf31b0f9c214f0abddcb569", // fathom-onboard codex-prompts 2026-07-23 3e4f8ea2
	"sha256:529a5e25af544a5b5e588494538259b4118361f7cb6851669c7a2fb9e16b6511", // fathom-doctor claude 2026-09-29 e1700dcd
	"sha256:5ba186534870ffa6dc31d2c3aa827eb7b6f2312c0121f3f4af0c66f3324f089c", // fathom-doctor claude 2026-08-25 2d070017
	"sha256:5bb37cbf6bd9a1748eca05a77cc24f1a8a9fe625237a19f6d3e361cb819cdc63", // fathom-custodian claude 2026-09-29 99942074
	"sha256:5e76a84cce60dbe70b9090424c1bef51a82dd2fd903089ab6250c888db938185", // fathom-doctor claude 2026-08-13 17158885
	"sha256:5f4117aa3da3d1b2d9c4574427218822d01fb6a23db3436e3e91a6c4e97abe30", // fathom-doctor claude 2026-09-02 9c1d9b16
	"sha256:5f54a5a2cccd0a6ca5325b364b5010c81991b8ce201d9d751014afff25be620c", // fathom-doctor claude 2026-09-28 189db126
	"sha256:615743018280e1a838fc6f107119748bd448f45cfcc5fdafd89fd3a3963fa52e", // fathom-custodian claude 2026-09-25 fb5ccfbf
	"sha256:624e8c6b2f5338f62e30f56c9320ee111ccb98f1b1fdf608da7c2d83fef14ea7", // fathom-doctor claude 2026-10-09 h1-guards
	"sha256:626494fe52b767899609f3aae24651e442f9686f176e5eeda1f313ca24b6b720", // fathom-feedback claude 2026-08-13 17158885
	"sha256:63636de86832ace2be296fa6637dc3293ca391e4de6e46f691640fed3ebdba9f", // fathom-onboard claude 2026-09-28 206cb920
	"sha256:639c0878af3d477cb67315ca7f6fed978a93a06a5a5511593c49957ef682c16d", // fathom-plan claude 2026-07-23 3e4f8ea2
	"sha256:6679a46b45d40c599e3d9e5e38e9bfffe59dd75ca8a28bb54e07070ca57be403", // fathom-activate claude 2026-09-12 f7227af0
	"sha256:66cd6f5dd5ec1091f9c50a268cc7fdc580339479e31006ee0ca0b450ee705cff", // fathom-rewire claude 2026-09-29 110eb2ad
	"sha256:67688dc6e7a8799dc34407d75aacf1caf1dda95c6202a3edbbb4ee9e00015979", // fathom-rewire claude 2026-09-02 9c1d9b16
	"sha256:693ec2ed5cc8fe256a1790a13f183a6f70946710cc666368e4e72677dea529ea", // fathom-onboard claude 2026-07-23 e5a2ac66
	"sha256:6a6fe63d2494906da065e094f3f4f0239725fa954aab8dddb634f6fd762a397a", // fathom-rewire claude 2026-08-13 17158885
	"sha256:6b57b61d873b8227628e510f7f644334b86aaf6bdf1a5448f9090d6767e76df7", // fathom-impact claude 2026-07-23 3e036c11
	"sha256:6bfd59a2b89b6df6ff367ff99ac0acdead8565b08a39d3c5123e02bf3a62c727", // fathom-impact claude 2026-09-27 194cce67
	"sha256:739ac4fa4552ae06c1184a560fa89e80d8cf2b3b1592e8dd7a20c9685667fcbb", // fathom-verify claude 2026-08-25 2d070017
	"sha256:75a0eda65d9a51fd665af45ba808bcba8a35c571f2b07c306360e1610323488d", // fathom-rewire claude 2026-09-28 f40b36b2
	"sha256:77b86073bbe1f4059fe42a874ac8a11185eacac9d4b8a1da05a2da7695b72b2e", // fathom-feedback codex-prompts 2026-09-02 9c1d9b16
	"sha256:79733f6776e3ed7b2c0587badeac3297ff4433b17059b7591d8c3623f1cd6629", // fathom-rewire codex-prompts 2026-08-07 1030ad40
	"sha256:799072638a7e4d78040356de6ef0fbde997d59ef040b0f5ab7be43ad1762e14f", // fathom-doctor codex-prompts 2026-07-23 3e036c11
	"sha256:7c90b3cf09e2ff9d2ca1c66c73150641c85ac1d1eaa4697b3a772e8bbde99f56", // fathom-doctor claude 2026-07-23 3e036c11
	"sha256:816a79e6e85a38599ffe78421b8fdc3b7f64647f5c9fe1033402aeb9c87b36e1", // fathom-onboard claude 2026-10-03 6a7c8087
	"sha256:87067b17ae83dbef46ba5364eb9273a9ed449f79761a0f0d0b475a6973562cee", // fathom-onboard codex-prompts 2026-08-25 2d070017
	"sha256:893461016186e2d4910b8d8a01b055d227bd8cd45c9a10c51c9eb9ac4b0a7c66", // fathom-feedback claude 2026-07-23 b5176c15
	"sha256:8a160e793a8a44157733a1cb74f89e9afaaa22e3310db9015a089057ce27bc47", // fathom-findings codex-prompts 2026-09-18 023e9d72
	"sha256:8e00d50406f9b8b05c1afbddb0cd02ad12d7d8fa086b89f6d901d7befc1dcd54", // fathom-rewire codex-prompts 2026-09-02 9c1d9b16
	"sha256:8f7f757ef2efe6f44db7689fa640b889d69b7f0d840013b483e34030af4fd2ec", // fathom-findings claude 2026-09-18 023e9d72
	"sha256:8fd41692db47bdcee5f7fab5dceef541a29830859803c3480017fa287e68398d", // fathom-onboard claude 2026-08-13 17158885
	"sha256:90d58545bcc9ee7ceb49d17a7df44c8c4d61491c3007efa3ded20de68aba6f12", // fathom-plan codex-prompts 2026-09-28 ccd97a0b
	"sha256:950975e70ebf36e7938af49192ec376500b47c4db24c9c9c8a72306eb177ed74", // fathom-onboard claude 2026-09-29 110eb2ad
	"sha256:954a97a00b0a67efdaa852addd541e91be6a875adf944d2337d23f9305b533d2", // fathom-custodian claude 2026-08-13 17158885
	"sha256:9b41afdbfe5fe14784b752f8e672716f4303a42a2debb2517d9df3b5c68d4548", // fathom-doctor codex-prompts 2026-09-12 f7227af0
	"sha256:9b7559bc6d792f254ffc3eee100ab0a1c446a7a1113e33c852b926586921d88e", // fathom-plan claude 2026-08-25 2d070017
	"sha256:9f795b373db640efc6f32b2fd60d121c801706b1fbedc8db0f3343a70f4d633a", // fathom-plan claude 2026-09-28 de50dc5b
	"sha256:a37085ba55fcc75ed5c95b0503f589c96b61e6b3ebe66ce65f99db26077322c8", // fathom-verify claude 2026-07-23 3e4f8ea2
	"sha256:a373431c17ff9687c555b5181afa1db6f066e03cc5bbaae772b998957ab4c7c5", // fathom-onboard codex-prompts 2026-09-29 110eb2ad
	"sha256:a3dcc4e9462d0a019bf47abaa206405bbb275f33934dd291dd0a771166f7f0b8", // fathom-plan claude 2026-09-28 4a480644
	"sha256:a5bf45f48473be411e2d0ff14e6b441c51465c37eebae987903abe1e2f94a770", // fathom-onboard claude 2026-07-23 3e4f8ea2
	"sha256:a8077a7128452c035a50b0012b4a00009a69d2e0236a61396e2620460b48fbd7", // fathom-activate claude 2026-08-07 1030ad40
	"sha256:a83e4419ea74479ec854b4b6d0b56d5d811fb02e6ffa6b61d9504bfb299ebc7e", // fathom-feedback claude 2026-08-25 2d070017
	"sha256:a87e67b0f87f68e007ed3f8bc84b489e926789141787cf89aff36427ac699c9b", // fathom-onboard claude 2026-07-29 c8200b05
	"sha256:a9aecb06bc04805d25c2adb80b6dfb028f8aeb26e06525d9f1341cbdb83da986", // fathom-rewire claude 2026-09-13 d3a5af69
	"sha256:ad29b9ffe899c7a7ff976b75ec4d5e525cdc3af739970f555b66175ca585157b", // fathom-impact claude 2026-08-25 2d070017
	"sha256:addd640e34c7b8c9540c4f99fbfefef2b804271f429c2994af1b0dc520e26510", // fathom-custodian codex-prompts 2026-08-13 17158885
	"sha256:af768d0f607b9ec9ca3195133509b26f028c1b24b2f8d5a4514f492b69f398a5", // fathom-plan codex-prompts 2026-09-02 9c1d9b16
	"sha256:b250a7f91f7c740d7e968330835b3b8c1bb1c5f8d79a2eb87b05d3ed78088b0c", // fathom-doctor codex-prompts 2026-07-29 c8200b05
	"sha256:b57046b667d6dae2bfeaa6ef70576dd5b1db484f6bc60258e25658e84234c221", // fathom-onboard codex-prompts 2026-10-03 6a7c8087
	"sha256:bb1f61dc375ebffe8ce81167e3cce0a4c3d5cd0fa04079ca4f26db2cf4329392", // fathom-verify claude 2026-09-02 3e86fac0
	"sha256:bc2f751fed360c97c8bc72b8b68632c84e7588f4afff1dda4ea174a406f77ee0", // fathom-onboard claude 2026-09-12 7bd3f13b
	"sha256:bd64414ed95293db19b9252fee1e43c9c4f87edca3567d04a5dbbbbee7092423", // fathom-activate claude 2026-08-25 2d070017
	"sha256:c0d2ad763a6faa4d1f77d7fa698a64d705d61f54f45f251c1c02e0922f6e258e", // fathom-impact claude 2026-08-08 9a2d0891
	"sha256:c25d02eb7d902168dca85955302cb975f37155e45d6ba0c9698ed414a7d821b5", // fathom-impact claude 2026-08-07 0fe95d83
	"sha256:c2d035349ff039e31b4e40db95bef346cd3c4edfec572640076df232c7a027b4", // fathom-feedback codex-prompts 2026-07-23 3e4f8ea2
	"sha256:c36527eec83bdbd811c5e699496baf4db17b51bed7a32c4764b2a96e785e3a62", // fathom-verify codex-prompts 2026-07-23 356630db
	"sha256:c3f1c663623e1691249085db72aee58349fd812d47586b7977e7ed7645f3f51e", // fathom-doctor claude 2026-09-02 3e86fac0
	"sha256:c66a02e2385d0f5f63106af0270eb1ec6c7d0352a6a432c6a4f6131a777dd87a", // fathom-doctor claude 2026-09-12 f7227af0
	"sha256:c9f78dde360e53f6745a049eb36c3ac0a143afdd5c1da8d0e557ca4ed8c5f1ab", // fathom-doctor claude 2026-09-29 110eb2ad
	"sha256:cb8db5e7359c77a75aa6d409d8e86ef560d206820fcadfde45482cef19c5d6b1", // fathom-doctor codex-prompts 2026-09-02 9c1d9b16
	"sha256:cd777d0ac3b9549d96c797f69cb0985fc6e0ccda707d0b8f3293d5a3a9cffbe9", // fathom-onboard claude 2026-08-25 2d070017
	"sha256:cd7fa80183a8ba86a87469e0de2fcba63020bfbc95c77447a133f3d81ec3b57e", // fathom-verify codex-prompts 2026-07-23 3e4f8ea2
	"sha256:d92b364606dbc750ad3175064f99fed5a7af32c6bc28db3a8fdecf10f72b0336", // fathom-custodian codex-prompts 2026-09-12 f7227af0
	"sha256:d955978602435a8eb5caf2752884f1e1adce1afe6b59483290ac37f2cabbc315", // fathom-verify codex-prompts 2026-09-02 9c1d9b16
	"sha256:dcaa13280a5f13926e65148fbaf6bb1815f4e9fe8f4e928eac24739f2f3326d3", // fathom-feedback codex-prompts 2026-07-23 370300ff
	"sha256:e37b28febc135de8b719c6288f4a53f86722c2057cce725f8335e06690196c16", // fathom-impact claude 2026-07-28 8483a7b8
	"sha256:e72ff4586f9f4b9cc4f6e0fcf7de36de79471cb4a4ac00dcd6cc21c86619c23e", // fathom-feedback claude 2026-07-23 3e4f8ea2
	"sha256:e74fd3b8c2e46ffef82f7455acba14f6ef9d4345e80fa5d7d0924623ee86db6d", // fathom-doctor claude 2026-09-01 9cb90e61
	"sha256:e75e271622e5b8bfd25db840011dbe4c650c5e503ca8dd7b58b8be36f3b7efce", // fathom-impact claude 2026-07-23 7a59507b
	"sha256:e79afeb6fa6f27a1891475602e3f07855e1adb3774f544b5cadf3803b733fd6c", // fathom-onboard codex-prompts 2026-09-02 9c1d9b16
	"sha256:e8615087e6029ee9e07cb12b23743bfe92a6f44a5eed49dcfe8792ec03f94246", // fathom-onboard claude 2026-09-01 9cb90e61
	"sha256:e887eadaa9cbd96bc7e24e2d361f317278d4a2414891cfc10ca9b18655ec2aa9", // fathom-activate codex-prompts 2026-09-02 9c1d9b16
	"sha256:e9b32273c94373d28c995d78f1b50a9bb4f5056e734d8c795f5a69b5c9d746bd", // fathom-rewire claude 2026-09-12 7bd3f13b
	"sha256:eb66688a2ceb0dcfcd47be8984de99dc51201c23c28cc7e156893ba723b0ac67", // fathom-doctor claude 2026-07-29 c8200b05
	"sha256:ee9344a23047f5e7740ad5647fbf0d59aafebb1b0ecb1ad56fecb3b9e3ef826b", // fathom-verify claude 2026-08-13 17158885
	"sha256:f1138ac9fca5b5f6792ffc8627a567684c8bc33ea794bfa6f05b9568c2c0b02a", // fathom-custodian claude 2026-09-12 f7227af0
	"sha256:f2448d2b35465350bac1d766f4a3f7817ee3dceb8bd2a7e254a28a918a1c5bdd", // fathom-onboard claude 2026-09-12 f7227af0
	"sha256:f35512c78d3a713d2bf28185cef0c23a185e6884649fbd13ecaa48c9d4f26fb4", // fathom-verify claude 2026-08-25 a41e2c2f
	"sha256:f3580d82c3aa15a4e51b002c93b52587bf4336046bae41681fe509dfe1faa02e", // fathom-doctor claude 2026-07-23 7a59507b
	"sha256:f49ae582bb5746bcbcf3cbac62d0df650cbf81340317aeeeddcfca6ceed2d5bf", // fathom-onboard codex-prompts 2026-09-18 023e9d72
	"sha256:f6748afe64f1c07c44bf5b90c9c5264d285f4eee1ee2ca1e4a272d63c3005aa1", // fathom-rewire claude 2026-08-25 2d070017
	"sha256:fac69dd48c9b18672de2e685cc98e438d48cdeeb2b82df294208c7c272c712a3", // fathom-activate codex-prompts 2026-08-07 1030ad40
	"sha256:fbd5e792983bf39f15c733e43c568a24951565a4266ae1dc5d45da02249ca064", // fathom-onboard claude 2026-10-10 h4-cgofree
	"sha256:fd0f9103ad4e3c7dbc733e8a34de1b6e52551107e6ff740dca194a6727f07b67", // fathom-plan claude 2026-10-03 6a7c8087
	"sha256:ffca1cf580f5acfe79f4ee95c8d55db282fdbc03eeff5cdca5192b2c8e17bfdc", // fathom-verify claude 2026-07-23 356630db
}
