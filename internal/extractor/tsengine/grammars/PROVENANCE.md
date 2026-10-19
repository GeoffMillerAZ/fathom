# Grammar blob provenance (ADR-021 §4, docs/plans/extraction-treesitter.md)

Every `*.wasm.gz` file in this directory is built by `build.sh <blob>` from **official,
upstream tree-sitter sources only** — tree-sitter's own core C runtime plus
each grammar's own C parser — statically linked into one self-contained
`wasm32-wasi` module per language via `zig cc` (musl libc, no Emscripten). No
third-party prebuilt `.wasm` artifact is vendored; no unverifiable blob is
embedded.

## The lock and what it pins

`grammars.lock.json` (LB-F12, docs/design/language-breadth.md §8.1) is the record:
for the core and every blob, the repository, tag, the commit the tag points at,
the tarball's sha256 and the SPDX licence; for every blob, its entry points,
grammar directories, ABI, the sha256 and size of the module and the size of the
committed `.wasm.gz`. `TestGrammarLock` fails when a committed blob and its row
disagree, when either exists without the other, or when a licence is off the
ADR-048 allowlist; the engine refuses at load time any blob whose bytes the
lock does not describe (I55). The tables below are rendered from the lock by
`RenderProvenanceTables` — edit the lock, then run
`go test ./internal/extractor/tsengine -run TestProvenanceRenderedFromLock -update-provenance`.

<!-- BEGIN generated from grammars.lock.json by RenderProvenanceTables; do not edit -->

### Pinned sources

| Blob | Repository | Tag | Commit | Licence | Entry points | Tarball sha256 |
|---|---|---|---|---|---|---|
| core | tree-sitter/tree-sitter | v0.26.11 | `64402de2857cc197ecc4ca3bc144ea91fda7e72e` | MIT | — | `1bab01ed21464f3272665b9c60e39ee79f68da1333e80b23f2c9356569d06971` |
| `ada` | briot/tree-sitter-ada | dd5fa4cdb3aba91abc687aa68fb1431396fce6a6 | `dd5fa4cdb3aba91abc687aa68fb1431396fce6a6` | MIT | `tree_sitter_ada` | `f106303c124e64fd758606d08ed1a1f567d110e96f6a24462d41fb4010e46484` |
| `astro` | virchau13/tree-sitter-astro | 213f6e6973d9b456c6e50e86f19f66877e7ef0ee | `213f6e6973d9b456c6e50e86f19f66877e7ef0ee` | MIT | `tree_sitter_astro` | `0876aab720967ec051622593f558030ccc64ecc3c862707a41f89c8f197d9357` |
| `bash` | tree-sitter/tree-sitter-bash | v0.25.1 | `a06c2e4415e9bc0346c6b86d401879ffb44058f7` | MIT | `tree_sitter_bash` | `2e785a761225b6c433410ef9c7b63cfb0a4e83a35a19e0f2aec140b42c06b52d` |
| `bicep` | tree-sitter-grammars/tree-sitter-bicep | v1.1.0 | `bff59884307c0ab009bd5e81afd9324b46a6c0f9` | MIT | `tree_sitter_bicep` | `e0516d4a918c2b0124062c20b72291211c0de330f02209b468653625198e724e` |
| `c` | tree-sitter/tree-sitter-c | v0.24.2 | `b780e47fc780ddc8da13afa35a3f4ed5c157823d` | MIT | `tree_sitter_c` | `2eeb4db31f8fa0865e45488503d13403923bcb485a1bdb637abff8c42dd97364` |
| `cmake` | uyha/tree-sitter-cmake | v0.7.5 | `e997bd0b275ca525ce9befecedf5299031183661` | MIT | `tree_sitter_cmake` | `b1c664921d06a6609f2b4fdfbd6685baea35066873ae5b75e11be55317d281f9` |
| `commonlisp` | tree-sitter-grammars/tree-sitter-commonlisp | v0.4.1 | `9db594efb43574c5fe4a1f880568b1f0d0ee6057` | MIT | `tree_sitter_commonlisp` | `f9767782bf94e82b847094f632f2db95b342e0ab54b1b8c8234cc1d6ac6c69a5` |
| `cpp` | tree-sitter/tree-sitter-cpp | v0.23.4 | `f41e1a044c8a84ea9fa8577fdd2eab92ec96de02` | MIT | `tree_sitter_cpp` | `7a2c55afe3028f4105f25762ea58cc16537d1f5a1dcd9cca90410b3cd5d46051` |
| `csharp` | tree-sitter/tree-sitter-c-sharp | v0.23.5 | `cac6d5fb595f5811a076336682d5d595ac1c9e85` | MIT | `tree_sitter_c_sharp` | `9628b164369071019368618bdefa446f0aab8acaac47b75d5dfb209e93b8903b` |
| `css` | tree-sitter/tree-sitter-css | v0.25.0 | `dda5cfc5722c429eaba1c910ca32c2c0c5bb1a3f` | MIT | `tree_sitter_css` | `03965344d8c0435dc54fb45b281578420bb7db8b99df4d34e7e74105a274cb79` |
| `dart` | UserNobody14/tree-sitter-dart | be07cf7118d3dba06236a3f19541685a68209934 | `be07cf7118d3dba06236a3f19541685a68209934` | MIT | `tree_sitter_dart` | `b126efb8dfeeba8b71f0c6ae1f0cd09cf8b205560786b31d44cc16b283e782f2` |
| `dockerfile` | camdencheek/tree-sitter-dockerfile | v0.2.0 | `868e44ce378deb68aac902a9db68ff82d2299dd0` | MIT | `tree_sitter_dockerfile` | `8cbdf50838cc55e841aa9585a1a09e5b8c41454ae30ce0a93a7bd71adb140818` |
| `elisp` | Wilfred/tree-sitter-elisp | 1.7.2 | `c0417c751a3dcc8724d1f2f6d1437a99dee1e090` | MIT | `tree_sitter_elisp` | `6a5c3df34b5bb1b4fbc1386fc0f24a205ddfff0720801899c3d1b3e3593fa670` |
| `elixir` | elixir-lang/tree-sitter-elixir | v0.3.5 | `e2d9e6e0e76b0c436fa48a0b8c32a031d0cbdf49` | Apache-2.0 | `tree_sitter_elixir` | `7d8bf37949e2bea75a19d38491d7680ed1c9f0f5a41a5235832d718ec174c7c4` |
| `gotemplate` | ngalaiko/tree-sitter-go-template | aa71f63de226c5592dfbfc1f29949522d7c95fac | `aa71f63de226c5592dfbfc1f29949522d7c95fac` | MIT | `tree_sitter_helm` | `86bcd2cd462a2fdd94a60bb44b6f15e99b3d02177c5d631cbf45761bae482458` |
| `graphql` | bkegley/tree-sitter-graphql | 5e66e961eee421786bdda8495ed1db045e06b5fe | `5e66e961eee421786bdda8495ed1db045e06b5fe` | MIT | `tree_sitter_graphql` | `ff542fddb51b1f1c110bb36fa16703d2212eeae0c0d12a2e1bf813c5f5db9507` |
| `groovy` | murtaza64/tree-sitter-groovy | 2a6ddd558b6aa39c5b77d8db9fe9baf817486b2c | `2a6ddd558b6aa39c5b77d8db9fe9baf817486b2c` | MIT | `tree_sitter_groovy` | `00320c650fdf683337d8a1fffd2f2f36fd4606f26d77f154d3f0fb7b268a5892` |
| `html` | tree-sitter/tree-sitter-html | v0.23.2 | `5a5ca8551a179998360b4a4ca2c0f366a35acc03` | MIT | `tree_sitter_html` | `21fa4f2d4dcb890ef12d09f4979a0007814f67f1c7294a9b17b0108a09e45ef7` |
| `java` | tree-sitter/tree-sitter-java | v0.23.5 | `94703d5a6bed02b98e438d7cad1136c01a60ba2c` | MIT | `tree_sitter_java` | `cb199e0faae4b2c08425f88cbb51c1a9319612e7b96315a174a624db9bf3d9f0` |
| `javascript` | tree-sitter/tree-sitter-javascript | v0.25.0 | `44c892e0be055ac465d5eeddae6d3e194424e7de` | MIT | `tree_sitter_javascript` | `9712fc283d3dc01d996d20b6392143445d05867a7aad76fdd723824468428b86` |
| `kotlin` | fwcd/tree-sitter-kotlin | 1852ea17b7f60fb3f9d84e0b1555d56b46b39fb1 | `1852ea17b7f60fb3f9d84e0b1555d56b46b39fb1` | MIT | `tree_sitter_kotlin` | `61282c8d3d231fab7d6086d4d0c9ff284824f78d4a7a8d3993d49023fbb739c5` |
| `lua` | tree-sitter-grammars/tree-sitter-lua | v0.5.0 | `10fe0054734eec83049514ea2e718b2a56acd0c9` | MIT | `tree_sitter_lua` | `cf01b93f4b61b96a6d27942cf28eeda4cbce7d503c3bef773a8930b3d778a2d9` |
| `luau` | tree-sitter-grammars/tree-sitter-luau | a8914d6c1fc5131f8e1c13f769fa704c9f5eb02f | `a8914d6c1fc5131f8e1c13f769fa704c9f5eb02f` | MIT | `tree_sitter_luau` | `e60c16f73ee29f2e04f78e6c807a82c0b1de1794db01e2c92e6e59701c897eb0` |
| `makefile` | tree-sitter-grammars/tree-sitter-make | 70613f3d812cbabbd7f38d104d60a409c4008b43 | `70613f3d812cbabbd7f38d104d60a409c4008b43` | MIT | `tree_sitter_make` | `bd60f6d2950086ba5d1d81e0890fc3e4e71e4d276b58b8cc1e1e95c9609ce331` |
| `matlab` | acristoffers/tree-sitter-matlab | v1.3.1 | `f03d0347acd8bb05d4edd8c845ac1718729e1fad` | MIT | `tree_sitter_matlab` | `7d8e0f1131a17e21f576839a4f7a9a16c864d89cbc1536a774445d590e6be470` |
| `meson` | tree-sitter-grammars/tree-sitter-meson | v1.3.0 | `12edaf0dfef0a43bc74336a4432b2db1c65788d6` | MIT | `tree_sitter_meson` | `fe9923e14405edfc41d76a1ffa82c8a03d8cf42e8651459a57882a55c00c83ea` |
| `php` | tree-sitter/tree-sitter-php | v0.24.2 | `5b5627faaa290d89eb3d01b9bf47c3bb9e797dea` | MIT | `tree_sitter_php` | `0e73ad63dda67ac12c0e012726a4e1a9811c26b020a0a2dea3e889f8246d9cf4` |
| `powershell` | airbus-cert/tree-sitter-powershell | v0.26.5 | `d398441825243b00e317e87e1829b9d6a3e54ce0` | MIT | `tree_sitter_powershell` | `a949ac2bccd74b94b6199702c8b119cb353785e8a8828150a763b202ae8d4534` |
| `protobuf` | coder3101/tree-sitter-proto | 0.6.0 | `6c878d18628ebbff3474479d2fdd6d8ba1954c3e` | MIT | `tree_sitter_proto` | `5a6931ce9b0a40470ba642efd775a832f2edf5b1f4d274d3b68f3182c1c920ca` |
| `python` | tree-sitter/tree-sitter-python | v0.25.0 | `293fdc02038ee2bf0e2e206711b69c90ac0d413f` | MIT | `tree_sitter_python` | `4609a3665a620e117acf795ff01b9e965880f81745f287a16336f4ca86cf270c` |
| `ruby` | tree-sitter/tree-sitter-ruby | v0.23.1 | `71bd32fb7607035768799732addba884a37a6210` | MIT | `tree_sitter_ruby` | `e7e49577ddc1f2de8e42d42353b477e338c15bbb95b2558e123ddc13d88789f0` |
| `rust` | tree-sitter/tree-sitter-rust | v0.24.2 | `77a3747266f4d621d0757825e6b11edcbf991ca5` | MIT | `tree_sitter_rust` | `061e90a539a55a6aa65dceb0ad6425c50ab1a6e3e6d4ba430e2795ed4550f10e` |
| `scala` | tree-sitter/tree-sitter-scala | v0.26.2 | `b931fcc338390925eb893d70ad070033f5856ccf` | MIT | `tree_sitter_scala` | `347ea9f62880b1a32a12bf58ae040f35c866996e80aff5bf28797fa9a40e034c` |
| `scss` | P233/tree-sitter-scss | v1.0.1 | `ee500b9cae6e9a961fa41229ede4fb9fe56e5da3` | MIT | `tree_sitter_scss` | `387a4bbf7250521758ad51d51b8f7caf4edca409a0b8108bf1ba6d7b5fe9b34a` |
| `sql` | DerekStride/tree-sitter-sql | v0.3.11 | `7b51ecda191d36b92f5a90a8d1bc3faef1c7b8b8` | MIT | `tree_sitter_sql` | `1fe13cb1e50dd9da9f22aed3cb9430fc9dae05a734a6049926fc12f6d8ecd3ae` |
| `svelte` | tree-sitter-grammars/tree-sitter-svelte | v1.0.2 | `774a65aea563accc35f5d45fafa4d96ec5761f57` | MIT | `tree_sitter_svelte` | `5fbca1c9857ea9af125ab43ea417b33e81b8268ef8003286bbd4fa9519bd4f23` |
| `swift` | alex-pinkus/tree-sitter-swift | 35245fbfee2fccf16273c6f4299438fb76875970 | `35245fbfee2fccf16273c6f4299438fb76875970` | MIT | `tree_sitter_swift` | `aa4f76cd4c8287ede397993ee369c51c75d419c51fb785d84fbee9d4ddfc3aa8` |
| `tcl` | tree-sitter-grammars/tree-sitter-tcl | 850a72ab6436e06645b33b11cfa60cbdb04b1f01 | `850a72ab6436e06645b33b11cfa60cbdb04b1f01` | MIT | `tree_sitter_tcl` | `4d3bbc7db5fd3398af714afc3dd4a0b4aba0b372bf30192bc4cff00d5acf54fa` |
| `typescript` | tree-sitter/tree-sitter-typescript | v0.23.2 | `f975a621f4e7f532fe322e13c4f79495e0a7b2e7` | MIT | `tree_sitter_typescript`, `tree_sitter_tsx` | `2c4ce711ae8d1218a3b2f899189298159d672870b5b34dff5d937bed2f3e8983` |
| `vhdl` | jpt13653903/tree-sitter-vhdl | v2.0.3 | `a7cf493d07dbcf52ca287596451ae3fb8b12099f` | MIT | `tree_sitter_vhdl` | `0afd50c99d2ee6277c9b183a6fb160173cf8635b3e14900a8dfc3dbbd6e93378` |
| `vim` | tree-sitter-grammars/tree-sitter-vim | v0.8.1 | `3092fcd99eb87bbd0fc434aa03650ba58bd5b43b` | MIT | `tree_sitter_vim` | `93cafb9a0269420362454ace725a118ff1c3e08dcdfdc228aa86334b54d53c2a` |
| `vue` | tree-sitter-grammars/tree-sitter-vue | ce8011a414fdf8091f4e4071752efc376f4afb08 | `ce8011a414fdf8091f4e4071752efc376f4afb08` | MIT | `tree_sitter_vue` | `e62913c5709ed1e68616bb85aedb2641a0c35c6b6709986ab71487d54b16ad0d` |

### Committed blobs

| Blob | Grammar dirs | Scanner | ABI | Module bytes | `.wasm.gz` bytes | Module sha256 |
|---|---|---|--:|--:|--:|---|
| `ada` | (root) | no | 14 | 374,900 | 107,979 | `e5d4df79895a32b1849e6053207e259e674c46310b7f9f593726fe66117855b0` |
| `astro` | (root) | yes | 14 | 150,848 | 61,599 | `023b29f668dd5bd7309ef3d54d8ed855563a15f8c78bdc6ae106b16b1948bea6` |
| `bash` | (root) | yes | 15 | 1,364,058 | 262,984 | `f46602976271d9a23e93ca93d927df2ed50f57026de6f13558fd9c97c4dbb28d` |
| `bicep` | (root) | yes | 14 | 265,595 | 76,391 | `7ad95054456a77f7b992f6a6191a2774b99155d82867f0b4a80055cc0e274f0c` |
| `c` | (root) | no | 15 | 564,097 | 136,140 | `22362d4cc55c6ccc6bada0775a97fb86d3dc801de439b54d06a8c870604eca33` |
| `cmake` | (root) | yes | 14 | 198,511 | 68,052 | `2250d3af489e7bef560a788f0b14ef436bd94003b8eb2d7472aecea9c55c3e1c` |
| `commonlisp` | (root) | no | 14 | 839,432 | 158,920 | `f9cbfce7bf5a6723d457925aa258cd9e224ebeab8988f369b710fe7a11581c12` |
| `cpp` | (root) | yes | 14 | 1,935,648 | 420,154 | `b5ca4b0cd346bc1cd1060fdc2151125a11ad7695da595157667a5e8b30b03e34` |
| `csharp` | (root) | yes | 15 | 3,056,786 | 603,861 | `b0700502f93f8e6fa577c47faa1dfcd53ae8b7467d16c621102238e75de8159f` |
| `css` | (root) | yes | 15 | 250,838 | 74,133 | `e99413feca59f90b54899d5257f0068ea82fea71e30aeb55f6342a104c23fd2f` |
| `dart` | (root) | yes | 15 | 914,734 | 205,433 | `50a95508f349684542905dae851d05c6833f7ff517b2db2ecd34bf91219ca4e8` |
| `dockerfile` | (root) | yes | 14 | 171,375 | 69,325 | `81274400989b6ac9f9d9d0cc9618b0ca5045214742468279fdf089d214e5b6ac` |
| `elisp` | (root) | no | 15 | 176,723 | 62,982 | `f207321d3e54d07838a585601113055624294da210c90c9de637cf31a2a6b397` |
| `elixir` | (root) | yes | 14 | 1,365,700 | 229,991 | `77398b69666daf0227d9209a70b172374880f191b78031697027cd3a8caafc01` |
| `gotemplate` | dialects/helm | no | 15 | 189,463 | 68,230 | `b1a9b7c210be48df96ad4f0a0bec88f4aca1f30fe9cb4bb2e05c6e845181d61f` |
| `graphql` | (root) | no | 13 | 178,965 | 66,018 | `2562a48542af5bdffed0ba53aa178e883828c124aca5b082b9be47f3aca83017` |
| `groovy` | (root) | no | 15 | 761,792 | 171,362 | `9bfe78467984483569fd59a44dc85fba0f0115d1bb6a6466b32ab7106ece6bc3` |
| `html` | (root) | yes | 14 | 146,253 | 61,224 | `da122f8345c4c4a052cd71c55b7b34b5f4bde63cedfd6de3ad8aeb11c8099289` |
| `java` | (root) | no | 14 | 401,009 | 113,109 | `335ca5db18443f9ff6bbb917bccd3c79425991bdf1d3474b62a64169c58405b4` |
| `javascript` | (root) | yes | 15 | 458,741 | 114,833 | `7245ee58564f991827498e36b5ec14420b99759596490e3a74ed0993b24ed904` |
| `kotlin` | (root) | yes | 14 | 3,054,455 | 567,216 | `5e672dfa0997692cea7aa6b0f19be4a26a846d6130264604a9a42fa6184652f6` |
| `lua` | (root) | yes | 15 | 173,721 | 64,669 | `e43aedffa10b61ea16c0c30dc70d5daed934be28a8259b5bda56d7230a3713f2` |
| `luau` | (root) | yes | 14 | 213,773 | 72,266 | `c80e039ffb082509ce038a4021ead4da3e2b55422d4770d272fa918fd089dc8f` |
| `makefile` | (root) | no | 15 | 265,371 | 86,559 | `7d6266397d5544d992ad80a03f4333c0680162ed4a37877f84fca8aa11c6dbce` |
| `matlab` | (root) | yes | 15 | 482,912 | 121,531 | `64f6964fff221ee76611150b791dcaf04b8f7d8a5d77b5c4158646b9423a3c34` |
| `meson` | (root) | no | 14 | 233,708 | 78,038 | `e17a6126f64f1a172421dc7214e5c8b034e8988b5fe3a1707956399aec324722` |
| `php` | php | yes | 15 | 811,496 | 191,343 | `e74d8a8385c252d7d3ce19e47247e48cbee4af9f30e39a6cffe0abe583a55881` |
| `powershell` | (root) | yes | 15 | 841,031 | 165,241 | `290c26ac0058dbc4eed6b79ef91031018fb5a892aa88b5a5b7e95beb9de01b46` |
| `protobuf` | (root) | no | 15 | 204,748 | 68,444 | `4211503d39d2cf30cbace8441e7bcced6421eb98617db0be18cbf992c3f92999` |
| `python` | (root) | yes | 15 | 539,848 | 123,581 | `ef1216ec19691690d3d237759d5ab8c3521e362423eef44f5e5234f7cb507ffc` |
| `ruby` | (root) | yes | 14 | 1,710,411 | 311,992 | `3299b506f05b73cb341de2cf65f1f327e9427e0149ce47d21f65af3d89b8924f` |
| `rust` | (root) | yes | 15 | 841,090 | 204,421 | `cf86df357ff237e6c95d24a5f5609e4f4cd45f1700d065ddae5d22099a99ee1c` |
| `scala` | (root) | yes | 15 | 3,125,460 | 633,687 | `11e9d547bad1ccbb22ff7bcaa49097d83ad5a576090d29de018ad9e29fdbd298` |
| `scss` | (root) | yes | 14 | 646,018 | 168,288 | `79723ff09561714719acd29ae3a833c5e620b54eaab15fb4a82b53a1d8b3fb7d` |
| `sql` | (root) | yes | 15 | 2,102,436 | 380,125 | `71d65822fb9af6262dcd4c3f9b54fd24d1b834609ecafd4d75e19ef8be71d4fc` |
| `svelte` | (root) | yes | 14 | 171,044 | 67,457 | `b3180077835cf81f8570e938c853ce6e41dae0e33892626bc9e7f47579de99b9` |
| `swift` | (root) | yes | 15 | 2,435,516 | 475,154 | `4e464eadd1f93f6a619ea2d5e1c26573a777cc7a37e7aa146495a82d3c3b04b4` |
| `tcl` | (root) | yes | 15 | 203,872 | 74,046 | `546a4170f4042ad33928b7b8b5cf5f3a2f76578627f763991bdce5063bcda183` |
| `typescript` | typescript, tsx | yes | 14 | 2,037,040 | 422,906 | `30080208c90581bf3b29b412e9ee2e9a11c89b36153b6a94cc07da7b5000afcb` |
| `vhdl` | (root) | yes | 15 | 1,314,192 | 240,100 | `3ec7500d69a8177a073f8c42e01fc4b40b4ac7c0259654e781375b3223189a1c` |
| `vim` | (root) | yes | 15 | 1,177,166 | 212,656 | `fa067ead3ba63fb2610653385665a94ace0a5d56b4cdafcdb8fa68080a7bf3b2` |
| `vue` | (root) | yes | 15 | 155,696 | 63,858 | `d9389312f43124431ddd3cf3e0914f015ee4be87419e3409be5bf29066430e8a` |
| **total** | | | | 36,506,472 | 7,926,298 | |

Toolchain that produced the committed blobs: zig 0.16.0, wasm-opt 129, tree-sitter-cli 0.26.11.

<!-- END generated -->

Before the lock, this file recorded the javascript and python v0.25.0 pins as
the annotated tag objects' ids (`f76aea6…`, `d326e4c…`); the lock records the
commits those tags point at (`git get-tar-commit-id` of each pinned tarball).

Grammar versions are pinned to match the CGO grammar versions already in
`go.mod` (`github.com/tree-sitter/tree-sitter-<lang>`) so the calibration
comparison between the incumbent (CGO) and this engine (wazero) is
apples-to-apples — same grammar generation, different execution mechanism.
`kotlin.wasm` has no CGO incumbent to calibrate against (wave P2,
docs/plans/lang-truth.md): fathom had no Kotlin extractor at all before this
blob (O69/O81) — ADR-021 §4's "new-language breadth" clause, not a
CGO-vs-wazero comparison. Its grammar (fwcd/tree-sitter-kotlin, not an
official tree-sitter-org repo) is not matched to any go.mod entry, since fathom
carries no CGO Kotlin bindings. It was first pinned to the latest tagged release
(0.3.8); LB-Q-kotlin moved the pin to the untagged commit `1852ea17` (the
repository's main, `tree-sitter.json` version 0.4.0) because 0.3.8 left 226 of
the 1,363 Kotlin files of square/okhttp and JetBrains/Exposed with ERROR nodes
(16.6%; `fun interface`, multi-line getters/setters, and constructs that recover
badly) where `1852ea17` leaves 15 (1.1%). A commit-pinned grammar is a
`tag`-is-the-commit row (as gotemplate's is); the blob grew 57 KiB gzipped
(510,036 to 567,216 bytes).

The core runtime version does not need to match each grammar's own release
train — tree-sitter's ABI is designed to be backwards-compatible (a newer
runtime accepts older-but-in-range grammar ABI versions; `ts_language_abi_version`
is checked at `ts_parser_set_language` time). Using one recent, well-tested
core across all four languages is intentional, not an oversight.

Blobs are committed gzip-compressed (`gzip -9 -n`, LB-F13) and decompressed
only on their grammar's first compile; the embedded `.wasm.gz` total is capped
at 40 MiB by `TestEmbeddedGrammarBudget`. TypeScript's blob statically links
both the `typescript` and `tsx` grammars; Kotlin's is the largest (a large
expression/type grammar plus an external scanner).

Each blob also compiles in `budget.c` (this directory, checked in — the only
non-upstream C in the build): the deadline-bounded parse/query entry points
(`fathom_parse_bounded`, `fathom_query_cursor_exec_bounded`,
`fathom_take_cancelled`) and the scalar-argument node accessor
(`fathom_node_end_byte`). See that file's header comments for why each
exists.

## Build recipe (reproducible — `./build.sh <blob>`)

`build.sh <blob> [outdir]` reads the blob's lock row, fetches the pinned core
and grammar tarballs, refuses a tarball whose sha256 or embedded commit differs
from the lock and a licence file (top level, `queries/`, `src/`) that does not
state the lock's allowlisted licence, runs a pinned `tree-sitter-cli generate`
for a `"generate": true` grammar, then:

```
zig cc --target=wasm32-wasi-musl -mexec-model=reactor \
  -I <core>/lib/include -I <core>/lib/src \
  <core>/lib/src/lib.c budget.c <grammar>/src/parser.c [<grammar>/src/scanner.c] \
  -o <blob>.wasm -Oz -Wl,--no-entry -Wl,--strip-debug \
  -Wl,--export=<the C API subset internal/extractor/tsengine calls> \
  -Wl,--export=tree_sitter_<lang>
wasm-opt -Oz <blob>.wasm -o <blob>.wasm --enable-bulk-memory   # ~25-30% smaller
gzip -9 -n -c <blob>.wasm > <blob>.wasm.gz
```

and prints `blob_sha256`, `blob_bytes` and `gz_bytes` to paste into the row. A
row with `"predicate_export": true` also exports `ts_query_predicates_for_pattern`;
the five original blobs predate it and keep `false`, so they rebuild byte for
byte (measured 2026-09-27: `build.sh typescript` with zig 0.16.0 and wasm-opt
129 reproduced the committed module's sha256 exactly).

`zig cc` targets `wasm32-wasi-musl`: Zig bundles a musl-based libc for that
target, so `malloc`/`free`/`memcpy`/etc. are real, working libc calls compiled
in — no hand-written allocator, no CGO. `-mexec-model=reactor` builds a
callable-library module (no `_start`/`main`), matching wazero's instantiate-
then-call-exports usage. The output only imports three `wasi_snapshot_preview1`
functions — `fd_write`/`fd_close` (libc startup references, never called in
practice) and `clock_time_get` (budget.c's CLOCK_MONOTONIC deadline checks)
— wazero's stock `imports/wasi_snapshot_preview1` package satisfies all of
them with no custom shim.

Build-time tools (zig, wasm-opt, jq) are **not** a runtime or `go build`
dependency — the committed blobs are embedded via `go:embed` and consumed
purely by wazero (pure Go) at runtime. Re-running `build.sh` requires network
access (to fetch pinned upstream source tarballs) and is run on demand to
add or refresh a pin, never as part of `task validate`. A different zig or
binaryen than the lock's `toolchain` emits different (functionally
equivalent) code; the script warns, and the new digests go into the lock.

## Runtime behavior note: the pathological-input bound (critic B1)

On pathological input — `({[` repeated 50,000x (150KB, the shape of a
truncated minified bundle) — the original unbounded engine ran **52.9s**
(200,000x: 11+ minutes at 100% CPU) where the CGO incumbent finishes in
~49ms. Root-cause attribution, established by direct measurement during the
fix: the **parse** of this input completes in ~70ms — the time goes to the
**query cursor's scan** of the resulting error tree (45s+ measured for the
150KB input). The wedge risk is real either way: uncancellable wasm
execution holding the extractor's mutex.

Three mechanisms bound it now (each chosen against a measured alternative):

1. **Guest-side budgets** (`fathom_parse_bounded` /
   `fathom_query_cursor_exec_bounded` in `budget.c`): tree-sitter's own
   progress-callback cancellation, one CLOCK_MONOTONIC read (WASI
   clock_time_get) per ~100 operations. Cost ~2% corpus-wide. The rejected
   alternative — wazero's `WithCloseOnContextDone` — compiles termination
   checks into every loop back-edge and measured **~3x slower corpus-wide**.
2. **Host-side hard backstop** (`tsengine.ParseHardBackstop`, default 2x the
   10s budget): a goroutine+reaper harness abandons any guest operation the
   budgets fail to stop (progress checks arrive at a slow real-time cadence
   on exactly the pathological inputs — measured: a 16x-amortized clock
   check let the full 45s scan run to completion), so the ingest thread is
   ALWAYS released within a fixed bound; the runaway call burns out in the
   background and its module is then closed, freeing everything.
3. **Parser re-creation on any engine fault** (`TSExtractor.noteFault`):
   faults are counted per-ingest on `fusion.Report.TSEngineFaults` with one
   stderr line each; the affected file is fingerprinted (not retried until
   its bytes or the ExtractorEpoch change); the next file of that language
   gets a fresh parser. Recovery is never a fallback to the CGO incumbent
   extractors (fallback doctrine — plan M2 ruling).

The v0.26-vs-v0.25 core delta remains the suspected variable for WHY the
error-tree scan explodes at all (unconfirmed upstream); the bounds above are
version-independent by design.

## Path not taken: Emscripten/web-tree-sitter-shaped `.wasm`

The grammar repos' own GitHub Releases also publish a `tree-sitter-<lang>.wasm`
asset (built via the tree-sitter CLI's `build --wasm`, i.e. Emscripten). That
artifact — and the matching `web-tree-sitter` core — were downloaded and
inspected before choosing the approach above:

- `tree-sitter-python.wasm` (official v0.25.0 release asset) imports
  `env.malloc/calloc/realloc/free/memcpy`, `env.memory`, `env.
  __indirect_function_table`, and `env.__memory_base`/`__table_base`, and
  carries a `dylink.0` custom section — it is a **side module** that must be
  dynamically linked (Emscripten's PIC relocation scheme) against a **separate
  core module** at a caller-chosen memory/table base offset.
- `web-tree-sitter.wasm` (the matching core, from the `web-tree-sitter` npm
  package) is itself shaped the same way, and additionally imports four
  `tree_sitter_*_callback` functions plus `wasi_snapshot_preview1.fd_write/
  fd_seek/fd_close` and `emscripten_resize_heap`/`_abort_js` — a JS-runtime
  callback shim (for streaming parse input, logging, and cancellation) that
  has to be reimplemented faithfully to get correct results.

Reimplementing Emscripten's dynamic-linking loader (dylink.0 parsing, shared
memory/table base-offset placement, GOT wiring) plus that callback shim in Go
is exactly the risk ADR-013 already flagged ("no production-grade pure-Go
tree-sitter-via-wazero runtime — grammar `.wasm` blobs target the Emscripten/
web-tree-sitter ABI, not bare wazero") — confirmed here empirically, not just
cited. The self-built monolithic `wasm32-wasi` module above sidesteps the
entire problem: one link step, no dynamic linking, no JS-shaped callback ABI,
only wazero's stock WASI shim. It was validated end-to-end (parse + tree walk
+ real `ts_query_*` compile/match/capture) before committing to it as the
production design.
