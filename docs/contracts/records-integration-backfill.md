# REC-8 frozen lexical title-candidate manifest

Contract companion to records-integration.md. This is a concrete immutable write
allowlist, not native-parser verification or permission to edit plans now. The
implementing lane must prove exact correspondence with existing plan parser before
writing; mismatch refuses rather than broadening the list. Only top-level plans,
README/archive excluded. Source hashes bind every candidate to the stated base.

~~~json
{
  "schema": "fathom.record-title-backfill/v1",
  "base": "468dc705813382d584926887b451cbea145c76ac",
  "files": {
    "docs/plans/activation.md": "92344eb3aeaca7931f8c730509e5d4a537c2cf632ad81991f7aaf05a0b419079",
    "docs/plans/advice-delivery.md": "3c145036beb90a3d375b9a294fa394c3cad14f58bf015a2ef7cf7b054639a31f",
    "docs/plans/agentic-decisions.md": "da857d5b6dd693aab7048f07ca2ae3a82dbb8bc361eda53ad478d959d9486db9",
    "docs/plans/agents-md.md": "e30749b5f942ca128ca876d7efcd6e4a82b1ec7e839acb530efd4b8dd10a478f",
    "docs/plans/ambient-fathom.md": "3b901fd6e991f3c22901700122c379a40d86cff9992aa9419bc2bf00642761d5",
    "docs/plans/append-points.md": "f4264c823cbd13f84bc6255ae4166ad6deddd10a78a10d98b736be14e049e9b5",
    "docs/plans/bench-confidence.md": "0ca85a7641b1528a1bfd7d7224d315e12c001338c5f9f9e7584eec17ae63827a",
    "docs/plans/bench-prep.md": "26f1e7cb7b3ad264d45aa15a8f76f22f5ac1e914a2bdb148dbaa0f7d0470ebb7",
    "docs/plans/bench-public.md": "b188c828e3b9a9661043efd1c537c31f2f3412615fed3f89807e1d8fe1724db2",
    "docs/plans/bench-pyramid.md": "284baf158ae59615b1353b72d8cb98202c954ddddca3c2b43e367a238418bd4f",
    "docs/plans/boundary-enforcement.md": "f99632e52bbd459290dd97903076c750f8fa4de4afb47233cf84bd31ed7534df",
    "docs/plans/carry-across.md": "0172cc9012c45736a0b48b28eda987edbd6e7af4f4ee21b9da857fd55438ceb3",
    "docs/plans/change-and-compliance.md": "eb13c743052e5f19e1c2ccc5cd1df3dd5af84a612ebb12418e864fd85d4933a2",
    "docs/plans/change-hazards.md": "b501ab0bdd24a6309a7f375a1543683c11edb748e9d7675bcd21f0fd1377bb06",
    "docs/plans/claude-code-mods.md": "586c656dafe1d55dcdcc3fc37308822975fe9a0581cd3eac581d5f41f9fe18b1",
    "docs/plans/cloud-sessions.md": "2f6b2d3fe412f73bf334adb6688ad3b72e5b98a8ca0b29d098b63d7b10de5a7b",
    "docs/plans/context-bill.md": "b47ba54b4d1e99c928483585258ad136a8c158180256a62a76209afd3482da53",
    "docs/plans/decision-port.md": "ee6bd7c1b5adefb0f75fe7856b9df782504fefffd8c15ed5aa3d89b8348a3878",
    "docs/plans/dependency-licenses.md": "8dd0d4d57311dce19415a1ee83e7013a4f8f349de7b52a63d8ace3496faf3bb9",
    "docs/plans/distribution.md": "2da6e0bb54dcfde61a6b6ceb490ab601324aafd3edb5d32ff62c3020605e09f3",
    "docs/plans/efficiency-wave.md": "15f9c03a0f9d37613cbcf5c0b57d7db68262dfbff923632d91b8572d41770ac3",
    "docs/plans/exploration-maps.md": "677e2eccb9eed150b9a2f5ed942c14691fa0147890923a41b5610ec6e561daa8",
    "docs/plans/fathom-dir.md": "117f22476bc8460ed6ba9d54efa33d631f7231a12079ac4f1c76c4bd73fe493b",
    "docs/plans/feature-labs.md": "0816b2e084c98cc235f450ff3b5a7a9e0f69d1b6f52965af2de5ff30c52ef21f",
    "docs/plans/feature-map.md": "22ba5741fdf2d0544676f89e6d5a43dc98c7b82a8b1a05e226dcdfe9ba7b6282",
    "docs/plans/fleet-picture.md": "d7be041aa6c6fde3374f0d9386499f9901a5943cfa8c484cb06a8cd6ad98d413",
    "docs/plans/gke-wave1.md": "836f269955227e98f078b216b4da726e71ce75a50164fd6cd9f2019f0ca6352e",
    "docs/plans/governance-learning.md": "318ff2797afa112786944522e0b7e9de1a11509fb49e9fa264e24779e0c0f919",
    "docs/plans/iac-facts.md": "6800a9350def28409ad85945a93b80ca39c8eb23eebe1587b9518bc36a66d007",
    "docs/plans/incident-maps.md": "a0b3b3b6f963a44a8fb7cd1507760c4cc3cb6e50695c15de151b81b2f059181b",
    "docs/plans/index-snapshots.md": "3dd877eea6d75a5a3549ce187852d589505722040907be11353963003feb9127",
    "docs/plans/intent-v2.md": "3ffcc3363e58f05d4e903e9eb05250b378777785c3304cdf4dcea8c86b9a2b88",
    "docs/plans/judged-predicates.md": "83ce0ad65a2f4db9a912013d97bca437adf4066db2d1b5263b36a353b4ca9cb2",
    "docs/plans/judged-telemetry.md": "36454f2a39769863a46332f7833279fdcf6761f08aa2d97d5a4711f893e278e6",
    "docs/plans/judgment-foundations.md": "602a8e394b694ac2babb180c77935ece566fed757a87cf1f4ce9a88120b4d5ae",
    "docs/plans/knowledge-retrieval.md": "d57045b5c7f693aac6f0f680c0fe878586cc97b94a2bddf1444d04737eb03f87",
    "docs/plans/landing-evidence.md": "5f7c44969d3cb7ea6537477f1ccaafd68223e559a432f1cd557db90facbe5a51",
    "docs/plans/lang-library.md": "7c5cc59e17dabcc17fa187534a7b15d289b928032a77f9c91426893d92a40143",
    "docs/plans/language-breadth.md": "e6d4a62e7f36050f179a05105739820da084d6994dd7d347a3ac61cc65e142bb",
    "docs/plans/library-roadmap.md": "2ecb8a7c1cfe6161f28ea004f714accaa5c379f97b9aaad61120b47751e92d83",
    "docs/plans/multi-machine.md": "1c26f7b082348a8a56eeef1bfdba0c083410f02b73ac0f051262ab5ba10f068b",
    "docs/plans/observatory-console.md": "fd7e2abb3d8b3eb7bc4eb53bc996948c4b7e5bd64db18de6efa79f3fccb3dc9b",
    "docs/plans/platform-engineering.md": "576445ce959d4edf127651aad37e439c6824dd9dca044aa32a4780950d18aaca",
    "docs/plans/portable-receipts.md": "92498ae87a31f6301436b734c6f6abb49e9905a654276facc6d593341e5c3fb9",
    "docs/plans/progress-and-economics.md": "bbeca7caf12628a7754465e782b407fcb00b78bd8382900ea5cba2c41749062d",
    "docs/plans/project-understanding.md": "4f8e27bd50484a30a332db87b3c1cf87a12a309a3069a531f759631fadf4a74e",
    "docs/plans/proof-program.md": "a0c05f9853b1cc1b44cb1c113203c5f3d3318240cbaf1a91d283d27f4fdfbbf6",
    "docs/plans/reach.md": "cc72e81cf52d74f71d8e9ccfe16fb709385c2c90aa6ac7d03aacc583d1b0d8e8",
    "docs/plans/readouts.md": "d52a82d3fc424b5bb2b617ec31541a9f3eb244ffed7d33766392efc74a5811f7",
    "docs/plans/realignment.md": "2392810232371350c4319a06a28fc9d0a814e00a90e3b0a55d8f3fea3f0cda54",
    "docs/plans/receipts.md": "5019ab6dc42d62ff056ec89dd3dc78b06b6419b70e2db2f382fd5b01416929df",
    "docs/plans/refactor-conservation.md": "480f418a990f65f862a9343aa95e73cd7a0da68d71636499258077ae0a905591",
    "docs/plans/repository-facts.md": "49527f254c5e572c6862e69e6433b61679e424186eb9f678544d9164270cdf3e",
    "docs/plans/responsible-tenant.md": "fb441f759f7a4f961341c41b97abfa1e4df6a253895f4b9234a42dad820470c2",
    "docs/plans/rule-selection.md": "fec48b95ffee12742c9565887dec5101c71b9b9e522fa4f3101d8acb27148a73",
    "docs/plans/sdd-reconciler-v0.md": "30145848cdab04ad934d8a671899c7cb6155351c872201e88e34913f9de217e5",
    "docs/plans/served-not-kept.md": "4f6aee8c771bce809a76da108bea3d6cb7368f85829ea8e39e276235c8a8d6bf",
    "docs/plans/spec-interop.md": "02a18ecd24ed4c2db8220e8e37def1b900306d230e503482de7e72e3ad7a4667",
    "docs/plans/stewardship.md": "79ddcb71f85e24f2358a68e78e764c40372827897932f97b3227669584a33bb5",
    "docs/plans/strengths-bench.md": "2bed661b4774c8afce4db6aa18e6ea9edee94cea9e7d2ddb4076d372e0a45eb6",
    "docs/plans/support-matrix.md": "b31e22047c7d43ad8cbb0661e21d2b3e371bcd556e76e2fd72588bb0afab6941",
    "docs/plans/surface-reckoning.md": "2a84f56e76648ce24da0cad397906410b65707011802c3c9da1c32e32279ab14",
    "docs/plans/system-graph.md": "d450aa0a5c91b24c1c2e3bbaa284bfe7d9db6af2f6b8810b83025b9269d220a2",
    "docs/plans/telemetry-links.md": "f7b6fc1215d39dfe839a1a1a5f885ff1ec1f1987df4c94150d9e437110749d06",
    "docs/plans/test-certifications.md": "5f8372a72137719746e86974d33fec07f1db4741c57cf9318297a59c00cda487",
    "docs/plans/test-intelligence.md": "dc841711976406fe12a40077bf573e79814eeda68b3bd4a0693a8abba16791c8",
    "docs/plans/test-run-hook.md": "bac7d82ccb3ec2777e5259bbd9cb7a603dee832cc927314fc52bd63be2be3ad5",
    "docs/plans/typed-records.md": "970aa3a73dc45d1d36ac72885e64aa5ad174eb8554dce30dcf462043d998d4be",
    "docs/plans/verbs.md": "48e22cbd23977fe061f4d91f081600d3b43a319d67ecad842c1f0293415f9635",
    "docs/plans/well-architected-evidence.md": "05d54770e77dbd7619717a5c761f3988292acba53557c478caf11faf13fa4fc8",
    "docs/plans/work-intelligence.md": "946feaaf933d3765bc4ae4b235fddcad74a7f344513526ba1aaf34b988027ccf",
    "docs/plans/work-signals.md": "0d9c62f57980a7d750488038473405c606d61dc51f70e12dfb80ebd8964af55e"
  },
  "rows": [
    {
      "path": "docs/plans/activation.md",
      "line": 51,
      "row_id": "A2-3",
      "before_sha256": "eb495beca06b0bd5cb59ee3e0d0595746f502458fa6c9208bdc742b740e968e3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/advice-delivery.md",
      "line": 29,
      "row_id": "AD-M1",
      "before_sha256": "41b32345779fc4bb52300e72f73e6b15d0752cfcca66ad55a86e73d42d9a126c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/advice-delivery.md",
      "line": 30,
      "row_id": "AD-M2",
      "before_sha256": "553cfb286186902b00b2e2e041a3fe180b206f845bfb0ccd6609b60d15b6c3dd",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/advice-delivery.md",
      "line": 31,
      "row_id": "AD-M3",
      "before_sha256": "c5b0e6552bf5da0ae0bfd16141689c13f00c292078b92467a6394f90fe5abfd9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/advice-delivery.md",
      "line": 32,
      "row_id": "AD-M4",
      "before_sha256": "3d61f0aed3267aa3451fb1c396ce3b4ebbb293a0901ef505a1f4128b4d8c4b42",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/agentic-decisions.md",
      "line": 23,
      "row_id": "EXP-1",
      "before_sha256": "9f7d9611de8a2d4af23599bb038c61fc4c4a01e6e7397340f49f0b0d08b7f1a2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/agentic-decisions.md",
      "line": 26,
      "row_id": "EXP-M1",
      "before_sha256": "68038aa2c4431e5342bd148221104386b1f1df05cbd4bcb4e8afe7fd4193429c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/agentic-decisions.md",
      "line": 27,
      "row_id": "EXP-M2",
      "before_sha256": "d674db6c0a0f97b296411deb9ceeae0fbb48861250538ff3cb9282b424d6a13d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/agentic-decisions.md",
      "line": 28,
      "row_id": "EXP-M3",
      "before_sha256": "7b68747026238ebbb403032f8f7410aeec139f0f5d35baa84b6048d4583cd5ee",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/agentic-decisions.md",
      "line": 29,
      "row_id": "EXP-M4",
      "before_sha256": "4f3294f493291655087f16b1654cf20ad5f0c9e8e986763d530348cc12c32b29",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/agentic-decisions.md",
      "line": 53,
      "row_id": "EXP-M5",
      "before_sha256": "95a72a6a4927768e7f6f9b87e7901e3a4685fa62c8981f7681ec9499aa638fa7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/agents-md.md",
      "line": 52,
      "row_id": "AG-OP1",
      "before_sha256": "de4a227c0d0acefaa1c77ad3d2147252ea25fa5e0ae58b3df505bdc0fafb229a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/agents-md.md",
      "line": 53,
      "row_id": "AG-OP2",
      "before_sha256": "9f67d0cf139357e5b1f9bfc719cbc5caa522351b4bc8a14f4c704457792f946b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/agents-md.md",
      "line": 54,
      "row_id": "AG-OP3",
      "before_sha256": "a744838a34eca501887cf7c30de3a93cfafa7dd278ae5e35d0b6930042757745",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/ambient-fathom.md",
      "line": 173,
      "row_id": "OP-1",
      "before_sha256": "a2a833f654e5bf2d921592dbe9e9211bb93acdbd1a5fd581e146d9aaf1e8fe60",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/ambient-fathom.md",
      "line": 174,
      "row_id": "OP-2",
      "before_sha256": "4c66fe47757a364c8603eb885ab1e3b77264a6f9f71fb04a2341e97ee39c2444",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/append-points.md",
      "line": 52,
      "row_id": "AP-M1",
      "before_sha256": "d2972e06e00bb37d6243263b5ef13397776133c71ef58e05eda4cacee9ec3f9a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-confidence.md",
      "line": 60,
      "row_id": "BC-14",
      "before_sha256": "ddd383fc1f9496085c7f0f4990cd7d7bb707051f174647a030555e8a95cba0ab",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 58,
      "row_id": "BX-41",
      "before_sha256": "e128ed05e80bccdc8d7c87bfead8f9065de6428c7196acf22bfb10bff2a07fd5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 59,
      "row_id": "BX-4",
      "before_sha256": "ddeff0c0b9d3db8dcfa2022dd7c77643e1f6e35e4e35ae24ff27f410238f390c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 60,
      "row_id": "BX-42",
      "before_sha256": "3595da0ece672bda7c0cbe423a3249f903cbba6651374c647ab77ae17f6ccb3f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 61,
      "row_id": "BX-43",
      "before_sha256": "a160849556dcf74dfe612f9aa7e66f144004ebb0d1630dcc66cf85a8651c8cfb",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 62,
      "row_id": "BX-M0",
      "before_sha256": "0f84a9e3339cd4267ddb0e5409ab5be625a72615f9fe194242ad254ceba148c7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 69,
      "row_id": "BX-5",
      "before_sha256": "5840eef55654dc6ada46c52f8e918d047242de9d19474e1198c616e2b143f553",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 70,
      "row_id": "BX-44",
      "before_sha256": "f98bd8fbbb88710b3dba85c875109f035190309dbddc76a04005aa1492277f96",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 75,
      "row_id": "BX-45",
      "before_sha256": "e3cecb983eca567e1bf07ff10e30e4d2a4bab9e09d7c299111269dd69872ec3f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 89,
      "row_id": "BX-11",
      "before_sha256": "0d78690c82984e90d214ec609e2bdfcbe4d3aab0ee0451527931a56b0a40f792",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 90,
      "row_id": "BX-12",
      "before_sha256": "21418ef9e66c56c1ecbd9535497e7b99c5b856cbbd6c771acea99ccbf44bd170",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 94,
      "row_id": "BX-55",
      "before_sha256": "ff6f3b65ab19026677617bf788dffb49cd049c8034429b68a5c062da040be652",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 118,
      "row_id": "BX-65",
      "before_sha256": "2f76a7b0888dd7f5f43abdb21732439db3628d1dd0424e2a62641a49e20c8121",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 119,
      "row_id": "BX-M1a",
      "before_sha256": "9da934105c48363022bae01bddb7ff6c20b0ee38d6ba9744b6496060356dafdb",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 120,
      "row_id": "BX-M1b",
      "before_sha256": "1114a8a55ee5f9823fed245d840c79085ca14567c6498e2cd5cfec9d577e4bee",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 124,
      "row_id": "BX-66",
      "before_sha256": "64eac81c28e884e1d259add2eccbe62d616df72b8263f035c7f780f2f8d21d32",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 125,
      "row_id": "BX-67",
      "before_sha256": "fd663892826b1c7e636d89495b668fc91ba8103c51e0c0afe3672a4653803a78",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 126,
      "row_id": "BX-68",
      "before_sha256": "d25da3bca88e34e1a08a370b987d99754acbe71e4ed80a2fb528adf4d1e8d538",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 127,
      "row_id": "BX-69",
      "before_sha256": "d716e9cdeb1ee9ad825b7ed44ce4ca49963ab0861194248ef91bf629be0f6399",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 132,
      "row_id": "BX-R1",
      "before_sha256": "7c9c772daf139e6968a45d4c8baa8a4ee587dc4fa1ce79708e60193a458596a7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 133,
      "row_id": "BX-R2",
      "before_sha256": "e5c556f32f2ce0362790c6f2aa7671171d3c0bb9332ea9df564398f03728cc66",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 134,
      "row_id": "BX-R3",
      "before_sha256": "382313b32e7919f4307799cbf85b372566c10240fef41ca93c94a41157b74f16",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 135,
      "row_id": "BX-R4",
      "before_sha256": "27f8c2aa985ed8c114066fe86ef9e8cc262ba18bca50ab465f3a14fbd590d7ff",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 136,
      "row_id": "BX-R5",
      "before_sha256": "8b0947a1ae49a9ec6a457a6b4b489529b46f60c7c0f9047dcd32073abc91f322",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 137,
      "row_id": "BX-R6",
      "before_sha256": "53d0f0f7ee4152231608987fd7d0db2c136f3ee6d67c4994b76ea5a57674348c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 138,
      "row_id": "BX-R7",
      "before_sha256": "9eb988f62dfd3a3683f2447867767553c56b408a3d6f6f2ec93fdedca81b3014",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 139,
      "row_id": "BX-R8",
      "before_sha256": "6e74231375a6469872ae4f6cebd35f6eb5314793a6c1cb84aaec3304ce2ff49f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 140,
      "row_id": "BX-R9",
      "before_sha256": "afde7cc5d64af4894646c29ee366aaa2d16d8eaff7c6e9db305138beb051eeeb",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 141,
      "row_id": "BX-R10",
      "before_sha256": "cc2c8a8af7a7ef38fa58111658e2005a77f8ed8b935b71858580f97bad099e2b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 142,
      "row_id": "BX-M1",
      "before_sha256": "54cea6e80f3ff0d3cae1443c8efbe9a7ecd5370486af4c9bd67c6535f0c13f83",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 146,
      "row_id": "BX-70",
      "before_sha256": "1bd4b2bec846439382afebd751015c0057c11980cf131be278076e82bbcd7591",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 147,
      "row_id": "BX-71",
      "before_sha256": "6d5d4e60d09573d7086919935b3255f6c5e691a4103a1120f823fd3192e3f0a1",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 148,
      "row_id": "BX-72",
      "before_sha256": "ac404d3f012bb4e3b6704a56192bbbcba0837680e7eac34ff814a4dad0dd3b05",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 149,
      "row_id": "BX-73",
      "before_sha256": "e76cdc1ce760f441c7daa4c18e903c66af38883b649982e6b06afd64b1a0052c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 150,
      "row_id": "BX-M2",
      "before_sha256": "55e99c194e35cdd3981380f15088d54da47d6a49bfe6c5a54207aa08a8aa1ef5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 157,
      "row_id": "BX-75",
      "before_sha256": "77f3f3fec62bd9693c892c30d1f524ec31748fec05a8ae1ba377fc55a81755cc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 158,
      "row_id": "BX-76",
      "before_sha256": "f29236bd0c20543f2824909a43b1b89b0b221ee1aadb785cd2f727c75a84f81a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 161,
      "row_id": "BX-78",
      "before_sha256": "24cc645b6099e03fd755e44d69c4e7819c65340b0222f4c219d544cdb13afce5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 162,
      "row_id": "BX-79",
      "before_sha256": "79f45f2bced90121de301e37b4461fd7ca99f35edd1bc5b6d85e08e4a304a501",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-prep.md",
      "line": 166,
      "row_id": "BX-M3",
      "before_sha256": "0ea65c72a8a21f5e6e8a071d2d12c62aee87a4b85c6d8cddb234e8baed99bc68",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-public.md",
      "line": 43,
      "row_id": "PB-19",
      "before_sha256": "a586980e94e18c8e803fbca525d2172788f8de02e2551b4b9b5551c6fc343d9a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-pyramid.md",
      "line": 57,
      "row_id": "BP-25",
      "before_sha256": "d31660b7ca553ae46dcf4d30268f52587a3c6ccd48ca710e7bc0df6c4bca02ea",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-pyramid.md",
      "line": 99,
      "row_id": "BP-44",
      "before_sha256": "87ce368a99f9d349365664d11cda07aa9fb89a995cd29eee069289d842e9714d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-pyramid.md",
      "line": 100,
      "row_id": "BP-45",
      "before_sha256": "709a112a4d97b4dfa0264211f7aef0786affab90043ddc113af00e694d1c742c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/bench-pyramid.md",
      "line": 101,
      "row_id": "BP-46",
      "before_sha256": "35414e8845271bc44a174e1f2d03953c8028d4e959e4649e7329a34baf94f4c7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/boundary-enforcement.md",
      "line": 17,
      "row_id": "BE-11",
      "before_sha256": "a8731284a862591ef1b7526f6580f3e4a564aef11a215826d598113192ca38e0",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/boundary-enforcement.md",
      "line": 18,
      "row_id": "BE-12",
      "before_sha256": "8afa66d7608119d7c18156343ab086df69c166031c485a4f2465b525904deb51",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/carry-across.md",
      "line": 15,
      "row_id": "CA-V1",
      "before_sha256": "5aadd3d2a4b0d70966c08ed6b5909fcd1bb5f63b2462360739caa2ec97ff8d79",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/carry-across.md",
      "line": 16,
      "row_id": "CA-M1",
      "before_sha256": "85b256064e0006187249a190d71d60234edb12cc1ec6cd02d73e748bce6ce08b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/carry-across.md",
      "line": 24,
      "row_id": "CA-M2",
      "before_sha256": "b6adbc0800241cb0d9675f972c8b87948ef9e482099d62a84d619c614ea37f68",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-and-compliance.md",
      "line": 55,
      "row_id": "CX-M1",
      "before_sha256": "e366f1b932281fc5c98b89f53aa124f046c8bea738e7b94e0a0ede6ac06e5c5a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-and-compliance.md",
      "line": 56,
      "row_id": "CX-M2",
      "before_sha256": "00306cfe2d1e65a3044f94adb7a5388184ad091fd846f9801b181e07d076838f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-and-compliance.md",
      "line": 57,
      "row_id": "CX-M3",
      "before_sha256": "5e34ec31c13e93b623352a24ff7d21f69e653c811c1fd2813284cc7427080b23",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-and-compliance.md",
      "line": 58,
      "row_id": "CX-M4",
      "before_sha256": "e2d5080600cb878b7eb8422382c0bc2c7111a96e8067c6d3f577e31d5783aa2d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-and-compliance.md",
      "line": 59,
      "row_id": "CX-M5",
      "before_sha256": "3a321c63875711bcd4a4c6dc7b275c9371d6596a91ae8259cf35c5d1e57b644e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-hazards.md",
      "line": 19,
      "row_id": "HZ-0",
      "before_sha256": "94138a7bbd3ad09a8e8ac90e0d2a18e56fa55d0a44cb5528252716d3514a2722",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-hazards.md",
      "line": 20,
      "row_id": "HZ-1",
      "before_sha256": "ef07df34c0ff6df47777c925a9a856e5a9a140525d781c06b670af85d0d0efee",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-hazards.md",
      "line": 40,
      "row_id": "HZ-M1",
      "before_sha256": "53402a5588cd2223336a3a48c69bdbdc4baa98d03d7d0f3a0d239bd2261fda22",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-hazards.md",
      "line": 41,
      "row_id": "HZ-11",
      "before_sha256": "587b96c10185cda3c2364d4e01f12ee04bc836d81bf43ff7ca027748aa29d1c2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-hazards.md",
      "line": 48,
      "row_id": "HZ-15",
      "before_sha256": "919d85fafe31f5f11f4f0f90852e2ca76d7fb03f85bea491f8ef0234b73b6d64",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-hazards.md",
      "line": 49,
      "row_id": "HZ-16",
      "before_sha256": "3f1e3b7d2336dc2877e16d065781a8fbae4cbef3e42234255da195619145ab9b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-hazards.md",
      "line": 50,
      "row_id": "HZ-17",
      "before_sha256": "c3e26d1e60a929fc8ae537d7ae1364b8cbbcf417191bba6e113d78ecc8b10ed7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-hazards.md",
      "line": 51,
      "row_id": "HZ-18",
      "before_sha256": "429682643673e9d9a93ab1ace05ac0559790a9b7275079cec06aded49d1d3605",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/change-hazards.md",
      "line": 55,
      "row_id": "HZ-19",
      "before_sha256": "32e04761e9b1d178066cbd73cb781121412a7628792133406d4b1f76aad300e8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/claude-code-mods.md",
      "line": 40,
      "row_id": "MOD-M1",
      "before_sha256": "d1b488d2abaecbe3a3f37941569096b1a366028a1e14c897ce71d964dca46d33",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/claude-code-mods.md",
      "line": 57,
      "row_id": "MOD-M2",
      "before_sha256": "d35c8716bfb11d26a48f405913d9870086fefe710b6c67e315f5cff355f73006",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/claude-code-mods.md",
      "line": 58,
      "row_id": "MOD-M3",
      "before_sha256": "3e90cc0e140b332180de2a333054e21758ee8876b8ed3ab2799110892734c063",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/claude-code-mods.md",
      "line": 68,
      "row_id": "MOD-M4",
      "before_sha256": "806de809a1dfd67520e26c9c17270cdae6fb4633aba697d46d2b20d8aed5353f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/claude-code-mods.md",
      "line": 69,
      "row_id": "MOD-M5",
      "before_sha256": "a7085cbf24bde773f9488254e0e72ffe6b80ac834e49ab8244a5fdb046103b4e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/claude-code-mods.md",
      "line": 78,
      "row_id": "MOD-M6",
      "before_sha256": "33abccdc71ee68b8c213422989c4080f55015486e737d89595bf796dee536884",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/cloud-sessions.md",
      "line": 38,
      "row_id": "CS-M1",
      "before_sha256": "fdaa5e34b90e7bea5ca342fa592e3e6bf6a5af7a4213faf1c4e4dc2091952df6",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/cloud-sessions.md",
      "line": 39,
      "row_id": "CS-M2",
      "before_sha256": "03cb621518b25c3692ac0d26514858d6bb706587b15d1ea4c6713ecdb8a5cfe3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/cloud-sessions.md",
      "line": 40,
      "row_id": "CS-M3",
      "before_sha256": "17bf550ecdeeb1d9423bb7e0749d335b7970c24569ba0c1b47521e700b535ee7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/context-bill.md",
      "line": 6,
      "row_id": "CB-M1",
      "before_sha256": "51a74ce726e653fb13f0b17de8ccb1baa1043a5daf9d40b10011a0441441d54d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/context-bill.md",
      "line": 10,
      "row_id": "CB-M2",
      "before_sha256": "23b6835cfd582e0927e291452c33931f6f6550595dd87602dcf3ce95e0fa50fb",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/decision-port.md",
      "line": 37,
      "row_id": "DP-13",
      "before_sha256": "ae75a61564864355bfbd58184ab805e980b4bbb8bb40d99bd21d8ee2d7f07c75",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/decision-port.md",
      "line": 38,
      "row_id": "DP-14",
      "before_sha256": "1979c00388da38403f00475c0742cda7d712329e89d77883463fc942c63f0684",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/decision-port.md",
      "line": 77,
      "row_id": "DP-37",
      "before_sha256": "08ff4cc8507fe169405a740bdd4651841f4d24685d875e594e57ec0380bb343b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/decision-port.md",
      "line": 79,
      "row_id": "DP-38",
      "before_sha256": "b533b9fc40f73dbccffc390d986192edc76279dacee353a7c8d04a7fde9156e2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/decision-port.md",
      "line": 107,
      "row_id": "DP-42",
      "before_sha256": "7003a9749fcad5b99f2178876b195a235f1355df12ef2de5894f06104c8fed95",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/dependency-licenses.md",
      "line": 18,
      "row_id": "DL-M1",
      "before_sha256": "6d13c43bd007d8f7dc8b56f1d9b4475e2093edddcd6182d61471930dfd28a3cc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/dependency-licenses.md",
      "line": 25,
      "row_id": "DL-M2",
      "before_sha256": "618a2de87b23658fadb06228ae2cf334d30534f13e797a8d743caa1e856983c5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/distribution.md",
      "line": 16,
      "row_id": "DS-M1",
      "before_sha256": "5f5d15fd48be4e7836d4b9799fe109d9f40c2d1b5dcfb8b4fe7aef722ab23584",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 22,
      "row_id": "EW-0",
      "before_sha256": "7b5eca67cdcc01db40d66c08c697e0787b7a5e1746710aed1fa1991ac7fbff83",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 35,
      "row_id": "EW-7",
      "before_sha256": "340f431e71fcadb507fc8a6b47aef3fa5b5565ef268a226be10c47f0d54f395f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 48,
      "row_id": "EW-6",
      "before_sha256": "8121e6a949e0d21e955813c3aa50b3ddbdf6cf1892371af647e8c223afecfabf",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 73,
      "row_id": "EW-S1",
      "before_sha256": "31fa4083a56f277fd31d4bb5fe1f6dcb051d7b6ddec0eb6d368a4d138f636031",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 81,
      "row_id": "EW-S2",
      "before_sha256": "892a585b15b9fd0612428c0e280a13580d8d1516d19869649b302d050c94621b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 94,
      "row_id": "EW-51",
      "before_sha256": "5dac704b449b9a9e5e3f111425f9a385e5ea51ce4e0f71ef962b50da68153aa5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 98,
      "row_id": "EW-M1",
      "before_sha256": "6f44b4f5c61a3d3722e671ec678be788dc0a415e23fd0d7f1fb7112c96117bd8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 99,
      "row_id": "EW-M2",
      "before_sha256": "abd40274477308e8e75bc5d9c90183fbf0093978141cb204b8a69a14d397fd12",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 100,
      "row_id": "EW-M3",
      "before_sha256": "fd2e76487a4942be64cb7ba8687aea143fba20e7ddf1560a67caa38714bea0e5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 101,
      "row_id": "EW-M4",
      "before_sha256": "bd05b7c69c9a37488dde39f35aacab52d805b1ba81d308e9651d8ad9284fef16",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 107,
      "row_id": "EW-54",
      "before_sha256": "a0ac336534554d4ddb2ac0b9b65ca87480baa821e56bf7ef0b1d902117834e1a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 119,
      "row_id": "EW-58",
      "before_sha256": "6f38c59cdaa80cc9cded513fb318296db9aa6a280fc1b8e39483792a87047bbb",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 127,
      "row_id": "EW-61",
      "before_sha256": "e6698d2950e85d288c89982dcdb2c678a3878d77f0fde3cccdf7831d316da5f2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 128,
      "row_id": "EW-64",
      "before_sha256": "5966822d13a4c128d5e8200ae5dd2f0532c783f03519df0e974f0fb4da28807f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 145,
      "row_id": "EW-71",
      "before_sha256": "62db6edf54f39b888daad8531b7f9f51f8188d7bf1cb20286355aa77cceaacd9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 158,
      "row_id": "EW-75",
      "before_sha256": "2453d852f92a24bc366b96793d8eda7c423102cd76be80a2f415f1e907af332c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/efficiency-wave.md",
      "line": 159,
      "row_id": "EW-76",
      "before_sha256": "e3059f711d21af71e73f95bb0c18d28e4d6e1062c969ed5873d3dbb8005f15bf",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/exploration-maps.md",
      "line": 26,
      "row_id": "XM-V1",
      "before_sha256": "30cf54492443bc857f8dc5bcff190d86e7caee71bdd94d15e3bb881331c3f6d7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/exploration-maps.md",
      "line": 27,
      "row_id": "XM-M1",
      "before_sha256": "3cdf55e4ebf2645e6540f9f31d3c3c6e3c42b906bd8f34091645ec2c4c1d77a1",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/exploration-maps.md",
      "line": 47,
      "row_id": "XM-M2",
      "before_sha256": "b2bded4d8ac37c260a47eb0763411f214e9165fad0385ec9937b94008af65b6c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/exploration-maps.md",
      "line": 48,
      "row_id": "XM-M3",
      "before_sha256": "cc1aa15da4b00fb1d0273cb13926ea2592c00f9fe7c863e4d53579d4c4178848",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/exploration-maps.md",
      "line": 65,
      "row_id": "XM-M8",
      "before_sha256": "da15a899c865c255dca1e6789b0932ec18070c8f170f4b0b9611c8b857a9d703",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/exploration-maps.md",
      "line": 74,
      "row_id": "XM-M4",
      "before_sha256": "35f8e3b6b239fdceca101db5c92eaf578ad8cef8b01353a7a94dcc6cf67bf8ef",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/exploration-maps.md",
      "line": 77,
      "row_id": "XM-M5",
      "before_sha256": "7e6f9ca7b9c9cb013aebc1a96757abc1af877894653400f3db3322e846ca4f9d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/exploration-maps.md",
      "line": 97,
      "row_id": "XM-M6",
      "before_sha256": "da0d2826269f2d0eb558bbcb313e6cc6c57f1d4f0cc330c5ac774301212bfbfd",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/exploration-maps.md",
      "line": 99,
      "row_id": "XM-M7",
      "before_sha256": "a6d59b2d98b129b4464613664ab6fa8e935c72ae3b802f620deb834089012d5c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/exploration-maps.md",
      "line": 105,
      "row_id": "XM-M10",
      "before_sha256": "c11e77f7eb24d5668fbccc62662ef49af4ecf8d8ee428b969c533501ddc4c4f9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/fathom-dir.md",
      "line": 42,
      "row_id": "FD-27",
      "before_sha256": "63d57711c8274a55434159849e5c6167ec17f947c80caa2cda6e0b8b3354d332",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/fathom-dir.md",
      "line": 43,
      "row_id": "FD-23",
      "before_sha256": "d79b7532e244da95d972139cf2cfe926bdb8ae5ba0a1bd758c35bd0d296b38d6",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/fathom-dir.md",
      "line": 57,
      "row_id": "FD-M1",
      "before_sha256": "bd7a40f8c5196856c18f230b3dd2d6f43cf765e3ae8aa9325aeec16b88ee935f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/fathom-dir.md",
      "line": 58,
      "row_id": "FD-M2",
      "before_sha256": "26dc1696dadc97128ca70d24948d0f294749289f6a0d49e97ec9f09791984b92",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/fathom-dir.md",
      "line": 59,
      "row_id": "FD-M3",
      "before_sha256": "7b7885fae0e078a937fcde582a8c7dc426a62e3fc9db78628516dcd812dc0f18",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 107,
      "row_id": "LAB-M0a",
      "before_sha256": "3df008f9355f926932d3896cd0fef3c07b3d004013131a57bea3f67129563118",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 108,
      "row_id": "LAB-M0",
      "before_sha256": "56ff720ac42a335d33af32be0089449047c7fda401c7ff9785adf22c06a4d5a8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 114,
      "row_id": "LAB-37",
      "before_sha256": "f8f42b2daac1aebb3c38e930aa1b459f2e75026d514b1b92da9f824f40d7a1c8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 115,
      "row_id": "LAB-38",
      "before_sha256": "fbee9565858d7efcd34a11c52ac8a71e23be90756f36f7db397f1386c178aefc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 119,
      "row_id": "LAB-23",
      "before_sha256": "f7aae11522a9ee7e87951c5ba8948626b951f2bc0ca142333d6b6c1bb98011fa",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 120,
      "row_id": "LAB-M1a",
      "before_sha256": "58127db6b6ea2e367d0e1a8ae7dc05ceff9b6585653ada0280a8d2eef08c6dc7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 125,
      "row_id": "LAB-M1c",
      "before_sha256": "4036beb4e289f8ac6519feb0d77d431be13344f71034dbbb684a49af4ab3de87",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 145,
      "row_id": "LAB-M1b",
      "before_sha256": "8f375a85677f909cc19197fdf7808becee49ff4d69d9e45e9588e223b3a5905d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 150,
      "row_id": "LAB-M1",
      "before_sha256": "8e53ea00b14c56c14f26534c7dd8deab577d39e92938bfeeac3c713ea089f5da",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 151,
      "row_id": "LAB-M1d",
      "before_sha256": "80d1729f2aca80523417af13e9277bed41e91685c1cae99e9a268dd8c42519a7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 155,
      "row_id": "LAB-D2",
      "before_sha256": "5751681288caaee4307ff9c63a4d084045ae4a0c421929057ec418d0c6c2bf51",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 156,
      "row_id": "LAB-M2a",
      "before_sha256": "6ddc52205fc3422613f30be4a72e29829a2d8b0156c38878789b5b58d8dddda9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 157,
      "row_id": "LAB-M2b",
      "before_sha256": "0c81e905afab07b74c28bdb86ccb3426b9a936a88abed3c15d6ab18da854267c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 158,
      "row_id": "LAB-M2c",
      "before_sha256": "1a017379b27079e7cf6a3b4ebc6b4e2fa567397ccd39941bbb529d8024a308a9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 159,
      "row_id": "LAB-M2d",
      "before_sha256": "feab98ae01f17b0e8f6689af23002da2ba0ff79e62ffd16cc2cca32ee8dafddd",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 160,
      "row_id": "LAB-M2e",
      "before_sha256": "5a76274a3ffae74c6fb58a9f9bd3fd05f3a99209fba329d9594d03c39d85e0c3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 161,
      "row_id": "LAB-39",
      "before_sha256": "bca4f6afcd8ac9f65dcf580a72c782b91ed59599f77a6e782860de457cda73b3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 162,
      "row_id": "LAB-M2",
      "before_sha256": "293060466be5d7087d58763eb35b3e6e2f32593e1c3b6f3f2aefdde202a0d679",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 167,
      "row_id": "LAB-D4",
      "before_sha256": "1b70536741229f365d12538d6bb59ba2944ab2099e6f4997d100d8b3255bd187",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-labs.md",
      "line": 174,
      "row_id": "LAB-M3",
      "before_sha256": "ec8a3f38307cd0e4eea149172e7c91d7b6092162d524881530c9badb7c02330b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-map.md",
      "line": 60,
      "row_id": "FM-M1",
      "before_sha256": "a8cc0aa590cb45dd48d66058befb3f02529deb73fa3e4178df1f99b77a788b03",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/feature-map.md",
      "line": 61,
      "row_id": "FM-M2",
      "before_sha256": "2afe13c76a4f08c186f9137d781a62e16ba9b24796e6c8837a97164c7a37f014",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/fleet-picture.md",
      "line": 17,
      "row_id": "FLT-0",
      "before_sha256": "d7a42203fd9761bc05e31a50082156899a4c56137b8903994250f922e6084832",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/fleet-picture.md",
      "line": 42,
      "row_id": "FLT-9",
      "before_sha256": "e43a0716a62ad8f77f5db9fe437888149b2d19228232b9a4ffe4ac924138f9d6",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/fleet-picture.md",
      "line": 46,
      "row_id": "FLT-M1",
      "before_sha256": "4b9769b9af0019384da4bb80159471e7b65940e1729606a1fdc26012439527ad",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/fleet-picture.md",
      "line": 47,
      "row_id": "FLT-M2",
      "before_sha256": "55aca389050815974e0664b085f201d7faf938c13ddfc6e7b1d781ab05aa0be5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/fleet-picture.md",
      "line": 48,
      "row_id": "FLT-M3",
      "before_sha256": "a534d659423016b317e34d2271ed481e3f00d33327e307da0136bbfca87be2fe",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/fleet-picture.md",
      "line": 49,
      "row_id": "FLT-M4",
      "before_sha256": "dc04140c34a4c10e7ce5ed25eb71ea84e08a325c45c58b5a802524fc7b4d8bc8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/fleet-picture.md",
      "line": 50,
      "row_id": "FLT-M5",
      "before_sha256": "fc076e759148005efb3b5958a6c4ead8b8e1c7a9f7efc6be7184470b36806c7c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/fleet-picture.md",
      "line": 51,
      "row_id": "FLT-M6",
      "before_sha256": "882c963f15cbfb6ed20cf76b19f196f6c5cd93651342cf9f788c53dee3884dd2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/gke-wave1.md",
      "line": 56,
      "row_id": "GKE-W1-14",
      "before_sha256": "b1db04c2651260be8f00fd88760c47322252a0fb7cd4f86291c771f8d353c458",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/governance-learning.md",
      "line": 61,
      "row_id": "GL-M1",
      "before_sha256": "dc37c069a2fbe595e351b0c23dfbe94ad9456d337e038e33255b01b78a7b4fc9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/governance-learning.md",
      "line": 62,
      "row_id": "GL-M2",
      "before_sha256": "211d87f77761f070f2ee4bbd2dc4bb15117989f258777751fdff32e2e6958bd2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/governance-learning.md",
      "line": 63,
      "row_id": "GL-M3",
      "before_sha256": "670274f56881b4b17fe65be5e4eda81da1eaf0dc4801af1fe57a8c7edcb8adcc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/governance-learning.md",
      "line": 64,
      "row_id": "GL-M4",
      "before_sha256": "8658a6261cf552c8f7f412f7bf309024564e60c483c5e606d79dac451e65f436",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/governance-learning.md",
      "line": 65,
      "row_id": "GL-M5",
      "before_sha256": "908ee0e18eba415a4a570fab6c89f6fb6455e7c3cea813569a14608d12a83890",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/governance-learning.md",
      "line": 66,
      "row_id": "GL-M6",
      "before_sha256": "5ddd0a03f78a0ed8424bc71491cd3d04effb2ba98e60e4ccbe549bd9ebd3881f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 14,
      "row_id": "IF-P1",
      "before_sha256": "dde776ba5e4fcbc1d8070d9e6b2efff220b615a4d7d4f5ea7fb8d0e27223fa1d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 15,
      "row_id": "IF-P2",
      "before_sha256": "35ea01da833db5a5b016d4d144d2e07428cc5163f13a2eadfb67c466dcc09430",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 16,
      "row_id": "IF-P3",
      "before_sha256": "b73a4a7f37882ec80d06b22fd5d37b550c3b3f580abbc37bfe919a31dbf2aeae",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 17,
      "row_id": "IF-P4",
      "before_sha256": "5a6d1bb456e42c40fff1ece8b0fdf9eeb44eeebbc76e0e045247bcfe042deb8e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 18,
      "row_id": "IF-P5",
      "before_sha256": "408171455cf6810a9c0f90bf29ab432b6817fc0e43fe4fb0440515d12b395e43",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 19,
      "row_id": "IF-P6",
      "before_sha256": "018c47b505ebc5fc46a7028ff8bf77cdf476d09dee341c86c890eeb6bfaad61f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 20,
      "row_id": "IF-P7",
      "before_sha256": "4afb9b07379bd7da5afe8af76e549c579795344ea1133b242729e3d3ef01a120",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 21,
      "row_id": "IF-M0",
      "before_sha256": "536cd326cb378213fb39cd165dce4270bf32cc1f9acf5a74151d93f99d50388f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 22,
      "row_id": "IF-P8",
      "before_sha256": "230dc602c2d97b9966c543ee6cc9f3ca431b8be7b569a679fe079fa01d95c005",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 38,
      "row_id": "IF-M1",
      "before_sha256": "f38fae692f4b80eaa698faf591eec9b4ba564a22836285ec4caa85bc7adefcb9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 47,
      "row_id": "IF-M2",
      "before_sha256": "a5588f622abd8b3a0ff89fbe3767bcbbfcc0c244c053ca72ff9df4a3a74d8f3b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 48,
      "row_id": "IF-M3",
      "before_sha256": "31173eff7b13491e6292479376a874fbeca378ad202c9986bd55a680dbbef990",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 55,
      "row_id": "IF-M4",
      "before_sha256": "41bb37083edbec414f2f1b3ce2302eeed0b8df486daebe116edcbf23d2504961",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 61,
      "row_id": "IF-M5",
      "before_sha256": "2d54590668b14cc97243480d93eccfa99763be9af2a762467859bb7dc1c3bd9f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 71,
      "row_id": "IF-C6",
      "before_sha256": "09c127da76e63e77814fcee4d100d40daa790ea1104422caf95ba9c89f1cbdbc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 73,
      "row_id": "IF-CM1",
      "before_sha256": "384376b25bfeb127e7fdf2ea849d88c3d48c50d903a48f80710b8a95a1111e8a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 74,
      "row_id": "IF-CM2",
      "before_sha256": "71c085d870189d38a211201de44ff93dcb2c93077c3651c37caacc7caeefe898",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 75,
      "row_id": "IF-CM3",
      "before_sha256": "1dc5a3314bb25ab86477a0f1b797c5aa2931adfeb6b07f043516a375a3622c4f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/iac-facts.md",
      "line": 80,
      "row_id": "IF-M6",
      "before_sha256": "b3d4a3284bf898b70cb791ae18b44bc489326ff4b88ee800365de2488235362e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/incident-maps.md",
      "line": 20,
      "row_id": "IM-V1",
      "before_sha256": "bcc7f54a9ead554e8189e698cbdf9ea3f2758f58ec453043cbccda9e905b44cf",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/incident-maps.md",
      "line": 21,
      "row_id": "IM-M0",
      "before_sha256": "e2fea193a0c07bb2441ac1d7c598a89e4363afc6a397e0f95187c886852e68a8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/incident-maps.md",
      "line": 22,
      "row_id": "IM-M1",
      "before_sha256": "591b7ccbd51a62a1d0134924878884f7a96d06c8502e2f31b40f485b512398dc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/incident-maps.md",
      "line": 37,
      "row_id": "IM-M2",
      "before_sha256": "024d8767520473bd245476b5eb1a950f55a1deed20f9baffd41552730625464e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/incident-maps.md",
      "line": 52,
      "row_id": "IM-M3",
      "before_sha256": "cabefc5208782409152818bb5e7bda7caaef7c9575e08dec9263acf0773fbe4e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/incident-maps.md",
      "line": 53,
      "row_id": "IM-M4",
      "before_sha256": "acc142838b0d5fd5209fef6966739be85aa495d587e8ebf910a9f8f471e68351",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/incident-maps.md",
      "line": 54,
      "row_id": "IM-M5",
      "before_sha256": "f3edc3d044b029c1a0c4ed6323e9c1a0286c19f7541079053ae8f5d7e6cbb961",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/index-snapshots.md",
      "line": 18,
      "row_id": "IXS-M0",
      "before_sha256": "6928898de98e4bf9a65842391c1c17a71faff07f5d9a43ad86bd237551fe88f5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/index-snapshots.md",
      "line": 28,
      "row_id": "IXS-M0b",
      "before_sha256": "6103bf5185e10a26c86233cf4cda4b2ca4ac1ffbaa0dc723e20dc1fb413872b5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/index-snapshots.md",
      "line": 36,
      "row_id": "IXS-M1",
      "before_sha256": "62da164387ba2fb5ead1665ae0f561f213b37ec6e1ddb8f433b1bff4d324fde6",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/index-snapshots.md",
      "line": 37,
      "row_id": "IXS-M2",
      "before_sha256": "bb15d2ee6f59d98981423039db0bbcd0f226dff6de82dba5e80443643224d049",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/index-snapshots.md",
      "line": 38,
      "row_id": "IXS-M5",
      "before_sha256": "ee03c73667ecac70b75b5024520871df9ad716348020360b0bc4e4fcb9601bf1",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/index-snapshots.md",
      "line": 39,
      "row_id": "IXS-M7",
      "before_sha256": "e2fd2fd3eeceb6bff230bdbee2f253c8b104bbe519c642504cd8cd28907c8d36",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/index-snapshots.md",
      "line": 50,
      "row_id": "IXS-M3",
      "before_sha256": "459c567cbd53255d515046a907f11c03a4fb410415f53dae62ca44859a4bc5b7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/index-snapshots.md",
      "line": 74,
      "row_id": "IXS-M8",
      "before_sha256": "79f86d40afe87f12c128bf875460361ee19306c16cb27b6c25aad43ca3741ad7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/index-snapshots.md",
      "line": 75,
      "row_id": "IXS-M9",
      "before_sha256": "5ca8e88373d8a850c9747f27d289c91a95b794405941bcc5b0d7187ff7dbf2b9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/index-snapshots.md",
      "line": 76,
      "row_id": "IXS-M4",
      "before_sha256": "b2c9941a02fdb61a37d22d7808bee0fc3b6cf3737aab61e0834e2863a502a668",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/index-snapshots.md",
      "line": 87,
      "row_id": "IXS-M10",
      "before_sha256": "37cfb51dac04bcab2b983fd8164f0242c5fdfa9e92cd89da25794da99f2fa4df",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/index-snapshots.md",
      "line": 89,
      "row_id": "IXS-M11",
      "before_sha256": "a67fbb82540421a9f645b047795d324bc238c86362f8c2f869604be87272e369",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/index-snapshots.md",
      "line": 93,
      "row_id": "IXS-M6",
      "before_sha256": "0fdad9806373dec1c4871c8be1f3ffb49c7efdeffac839341cf965fa64d71bcb",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/intent-v2.md",
      "line": 43,
      "row_id": "IV-9",
      "before_sha256": "a3a87c5b251199883c2a32aa2174dbe340f633bfaf10c4f065469cd159bd8c24",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/intent-v2.md",
      "line": 44,
      "row_id": "IV-M1",
      "before_sha256": "7b4e3b91610f012ea457a36b3aea0039751c98a987c267ce3fab39968c3a90bf",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/judged-predicates.md",
      "line": 15,
      "row_id": "JP-1",
      "before_sha256": "3c61938fae3d2bfa3038cf1d5cc2ec7598370ecb6b60bddbebffc7e17f84af1c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/judged-predicates.md",
      "line": 16,
      "row_id": "JP-2",
      "before_sha256": "7793d7f0e69db2fdb18b553771aeaf358a00901d93fb5afabdb367ab39a23be5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/judged-predicates.md",
      "line": 17,
      "row_id": "JP-M1",
      "before_sha256": "a3dc4d80974bf13b865b93124f288c324804dc8774bfeaea4aafd68f8ff23658",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/judged-predicates.md",
      "line": 18,
      "row_id": "JP-M2",
      "before_sha256": "00c97d92978fc20a06d9d9d19f05a6abbdf1e3b24451233bc5941cf9f2638942",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/judged-predicates.md",
      "line": 35,
      "row_id": "JP-M3",
      "before_sha256": "c7bb41e8b279543b51cffca5543e6feb8eae6b4dc6361370251a28c589653831",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/judged-predicates.md",
      "line": 42,
      "row_id": "JP-M4",
      "before_sha256": "7c9a547cf0d15bfed6b631a73287da1bd2b0b9a70f13d387c1c847fef795c8bc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/judged-telemetry.md",
      "line": 19,
      "row_id": "JT-M1",
      "before_sha256": "e63c71b78145970b15e2a350fa61d1af6f5d51ae90fa8965d7686e07d0b2bd93",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/judged-telemetry.md",
      "line": 20,
      "row_id": "JT-M2",
      "before_sha256": "4c2f487d9ab15d548ca5c168ac08a2f393ed5e78a622223994fb881e589ec82d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/judgment-foundations.md",
      "line": 70,
      "row_id": "JF-M1",
      "before_sha256": "518b4d0f97d0dc4c9da2f9dd6af879ec1ecd2f995019a54b9e03d6bdb291849e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/knowledge-retrieval.md",
      "line": 47,
      "row_id": "KR-M1",
      "before_sha256": "7a1b154583fdc3a8ce0f7a640f0a5e9161eb90abe7dcf0016372ba8fac992a6a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/knowledge-retrieval.md",
      "line": 48,
      "row_id": "KR-M2",
      "before_sha256": "bdb2b6abb75543d9d4710d49145e5486c6f1446e1f1c94ca91f35e3a7ce722bd",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/knowledge-retrieval.md",
      "line": 49,
      "row_id": "KR-M3",
      "before_sha256": "d043eff3b45cb41d91e51ad8450f120d5bde7e251434cd1a0665c10751f2b19f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/landing-evidence.md",
      "line": 12,
      "row_id": "LE-M1",
      "before_sha256": "767e2cb316bbf8ffa813e3ba21fb64f35ab2036f293665742797951d5b0f4f9d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/lang-library.md",
      "line": 20,
      "row_id": "LL-M2",
      "before_sha256": "fb91198223bff89d95e573e8b099405b296768b52cab0abe8238ca5ec1bc6a89",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/lang-library.md",
      "line": 45,
      "row_id": "LL-M1",
      "before_sha256": "194270baebf4c2bd3039e6c2d407a372969831ac0ec63615bdf5d86bdd106527",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/lang-library.md",
      "line": 46,
      "row_id": "LL-M7",
      "before_sha256": "fa884fd1c7174db12acdce2f247342d4790c313e0472707e565855de85a93841",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/lang-library.md",
      "line": 58,
      "row_id": "LL-M3",
      "before_sha256": "0a937c1c44ba8496f17e617942bed49d0ded89705768d118eb9c50c577fd0312",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/lang-library.md",
      "line": 59,
      "row_id": "LL-M4",
      "before_sha256": "c93d436ec3186647e956a684ae827df1ee53db31044132c9fa86a3fbaab907ca",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/lang-library.md",
      "line": 60,
      "row_id": "LL-M5",
      "before_sha256": "a450596ca6a1d81ad91e46784b800f4632a7a2a62f5f26d2107f69ae9615ce45",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/lang-library.md",
      "line": 61,
      "row_id": "LL-M6",
      "before_sha256": "1e66d7be8ac2e35c05389fb648bc287ef36a6714fb42fcfb0c86265875b183e7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 582,
      "row_id": "LB-F325",
      "before_sha256": "af3d582194447fc265da1098a66092a860b5027102016688603c149055d16932",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 604,
      "row_id": "LB-F330",
      "before_sha256": "af7d4ee0082d818b5239288eb6cc4a9537a4120397338894707dbae728d38b7a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 625,
      "row_id": "LB-F336",
      "before_sha256": "d7aceef1eda8b49f50389ef47ac8e427c984084f0b4e6efb06c568812a6c92dd",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 640,
      "row_id": "LB-F340",
      "before_sha256": "192bbc6f15b6fd978529a8e006c51ea0bccad4cd4cd41a17e301b4c6f4633a85",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 657,
      "row_id": "LB-F345",
      "before_sha256": "b4f6b5f803a0bc142e8972538c9fe5c59631c2e7761cb373412c00d6bc0335af",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 675,
      "row_id": "LB-F349",
      "before_sha256": "edb9466c10f99092ed91589466fdf0e41535bcf76f60de403cbc1f002925aa28",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 676,
      "row_id": "LB-F350",
      "before_sha256": "c272d582ba039a9e119f6ad4ea02f91860de36f46911e308c155edc3d459f1d9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 700,
      "row_id": "LB-F355",
      "before_sha256": "6e5ded6209e219218e9f1708a30ded6245439f5678cf7fede6cc4ffbadb6edf8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 750,
      "row_id": "LB-F372",
      "before_sha256": "d0a267e7d752e87769fd0e1c02016938e684b14c1c35f1a835ecd8558d7cecda",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 795,
      "row_id": "LB-F380",
      "before_sha256": "c8b8bd8f91a44b617e99b852fb5426882a59cc7d53290993a9139815b1d7fc1d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 820,
      "row_id": "LB-F386",
      "before_sha256": "50dec9457acafdf44f5655387290815860615773260e48613a20bfcaa49e965f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 903,
      "row_id": "LB-F414",
      "before_sha256": "1a42cbd7e84ff57235ae2cc54b40fcb67152a1c70677720e950c5cce2f079810",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 904,
      "row_id": "LB-F415",
      "before_sha256": "8152712974383131dc3358b6bb7bb08fd01d063868aa99b47c62d50da19353ee",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 905,
      "row_id": "LB-F416",
      "before_sha256": "7b0ce50113fd948d8575bdfcbe512a98196adb261876e75b095dd6d21d3cf618",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1002,
      "row_id": "LB-F452",
      "before_sha256": "849a3207ce29f51be62dd22210f7cd9d65963a41a0e453299bfd287cede38faf",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1006,
      "row_id": "LB-F456",
      "before_sha256": "d3b338042157fe73b2062920c406573fbe3c47f1482fe05c26dbb71e521b0607",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1007,
      "row_id": "LB-F457",
      "before_sha256": "a26b421434b0e7deedd4d1cb6cceb381149830c4c28c83a7c4ce345b68e3f8c9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1008,
      "row_id": "LB-F458",
      "before_sha256": "db0c6b30525bdd690ae57ef5c3cdb647287b85279fe837cc2ef7035327891f5c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1042,
      "row_id": "LB-F462",
      "before_sha256": "ff7747463b090f82f9b93cba2a85b441f314172bd65ba6a2ebfaeb6c351ecba0",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1043,
      "row_id": "LB-F463",
      "before_sha256": "4bc8359441fa10ac410c45d29f404cc7e20a97cafc447a9f42e98633e00dcfe4",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1044,
      "row_id": "LB-F464",
      "before_sha256": "076ac6ed3d281751a070732bcb5f72f01ec83a5dcd9b9ef60a4cc051eb0cb680",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1046,
      "row_id": "LB-F466",
      "before_sha256": "c60ab6061d15cda9f2c1c11e28ffa34b6ea8c94850e701c1cccc8dfe8b14c692",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1073,
      "row_id": "LB-F471",
      "before_sha256": "19f45750afcc1339ea1bf6a332f46f5eb83727a566ecc299791a86126498ec9d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1074,
      "row_id": "LB-F472",
      "before_sha256": "bf4528b468a9909983d069b2fb102b992f806e8bb98807cdb0a92d89b6bcece5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1095,
      "row_id": "LB-F479",
      "before_sha256": "5a501fc445ce1433118c57848d1d87562bcee0993e9be2f84330eebf96d0743e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1118,
      "row_id": "LB-F482",
      "before_sha256": "45c2f1394d51f27bb03dc8e69807a32aedb8daccab93e40c00bb1bf3ad6a322c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1119,
      "row_id": "LB-F483",
      "before_sha256": "385a7f59f59b98a7170ea0b724c2cdaa2aa16d6048b2418069e6505f3e9332dc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1120,
      "row_id": "LB-F484",
      "before_sha256": "7067c880c037c60d56c4083f93a6933c33ed12bbafc3e69202759a7379e65854",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1121,
      "row_id": "LB-F485",
      "before_sha256": "8a3171cfb589c17c644541480a2ba6bd169b11d8b13f502b8eeff9f2b7035807",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1148,
      "row_id": "LB-F488",
      "before_sha256": "af427782be2f04b530ebdecf70f01e6eb5d61de45cf42f88a7cd8dc8a778b648",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1172,
      "row_id": "LB-F493",
      "before_sha256": "e73ecc0fb9e0c7bb1cfdc5db95c9376a6bfa0f95b7868583e8ac81fd1c9c79e8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1186,
      "row_id": "LB-F506",
      "before_sha256": "acc7988035df00f2e623a33ce3766f86398351db5f808eca7023c8a7f206d079",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1187,
      "row_id": "LB-F507",
      "before_sha256": "1e5226e9f6d35640470d11b1cb052464fdf9c96423be996cab8e3b2ffd4409e6",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1211,
      "row_id": "LB-F512",
      "before_sha256": "9a78a0fd40efd329f50f7b2093f1d758ad73687c13f98c9a6a5345161b8c47fd",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1212,
      "row_id": "LB-F513",
      "before_sha256": "531c23395da31e5f0fbaeb530bf7fb04fcf0b3052f73d2f3502599b80bfc4562",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1213,
      "row_id": "LB-F514",
      "before_sha256": "3ceef27dbcc1ba3a83e5a5ff11a8a9a98bf08d370b8e28c502c1e558e990148a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1214,
      "row_id": "LB-F515",
      "before_sha256": "f2c49ef16ef03df6375065d874dc7c2ee76bd80e10a4ff405895b5f9e9f84ae7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1215,
      "row_id": "LB-F516",
      "before_sha256": "5ca94401bd5e465228c4216e8327e40f5444653ae500d0145b47e73e544a717d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1216,
      "row_id": "LB-F517",
      "before_sha256": "82651d8c81ca87e45ecca0ef4ab9734bc21811d0b7871873dbabb2073659565a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1243,
      "row_id": "LB-F523",
      "before_sha256": "901587c34a2b64e13dcc61b58bb6693a00c734cd79702f631fb47d588855bc59",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1244,
      "row_id": "LB-F524",
      "before_sha256": "0650db9c6a5782a8947174c3cd0c2613da4ebb935615d7658255e567b2d3b463",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1245,
      "row_id": "LB-F525",
      "before_sha256": "1367b35cc48881f9cd129b6e4ced03e8088d1cf5f09540da89943301bd364293",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1268,
      "row_id": "LB-F531",
      "before_sha256": "24910a4264e002c869a182e13c117ca228280c3bf26151ab2f9b469517600ec9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1270,
      "row_id": "LB-F533",
      "before_sha256": "54cdfda2431801c2d988fbb71c5a84cd6705a3ffe0c1da02b22dcc321f670a1d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1271,
      "row_id": "LB-F534",
      "before_sha256": "9e757dc1f470108e69cf0a63a664236b8d6d615742235e10f0d2e51703f3bc36",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1272,
      "row_id": "LB-F535",
      "before_sha256": "ab10b181347b35735ff8b4c40b5504a1c5a8c6e9d099a250fec2dce1c2e7301d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1276,
      "row_id": "LB-F539",
      "before_sha256": "640ac0c338e4ba515b0a145d9bb0c4a235b79f94759f1a858df38d639db161a6",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1277,
      "row_id": "LB-F540",
      "before_sha256": "12c0a1aa322fcd98ec2935e85375f713cae0f2c674baf319d9c4e0b0a85c33bc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1300,
      "row_id": "LB-F541",
      "before_sha256": "a5c0f3e1296c750b025a22eb1b9ccedb969edb324befaf47a9c6a079185fcd6e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1301,
      "row_id": "LB-F542",
      "before_sha256": "f7772952f8a6009654792a80f3d8acf700d7aaa62952616073d74af490ade8e7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1302,
      "row_id": "LB-F543",
      "before_sha256": "a2b08fdec90bda01f4211eacf664350ffa5076bfce96119aa1cb4ad6395c41c1",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1306,
      "row_id": "LB-F547",
      "before_sha256": "c2eec1ab68f0db4c9fca9b2be398daa7a658f563166b2fa26a63c4092375deed",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1307,
      "row_id": "LB-F548",
      "before_sha256": "f231b4fb446b5cb038539241de3cc55d88e15428b534572bb440365be8570765",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1308,
      "row_id": "LB-F549",
      "before_sha256": "55cb404a0303534782836d5f5cd33cab0d9802e469f2ecd353db3fdcce3cd0a7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1334,
      "row_id": "LB-F551",
      "before_sha256": "f29f986111c6a7cf230792b0bc9a7270388d1a9f322426591a4faa6b70bace96",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1336,
      "row_id": "LB-F553",
      "before_sha256": "75e745083da15764129f99843da7c5dcfdf3afc9bd1b0c11e29fbd9cfddaa134",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1361,
      "row_id": "LB-F559",
      "before_sha256": "4956bafd33acfd21e2464dd9b6ff99e1359efde6cc3087f8528841d49a2d0eba",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1363,
      "row_id": "LB-F561",
      "before_sha256": "e8ab64a492967925584a96e7f2e73a5350e7d5a481df98e4eb5adc5ab0c3e57c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1382,
      "row_id": "LB-F572",
      "before_sha256": "8b936a499c0a20bbe68e4b717cce6df862a6283ae44a9ba4e81e678f095509f6",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1383,
      "row_id": "LB-F573",
      "before_sha256": "f32c58d62fe655bbab2c84bed2d1703a2eed15f93d045016d697a56cdda665ff",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1387,
      "row_id": "LB-M1",
      "before_sha256": "9f0efa1d06ff0221f2473f49eb4723621ed024f06c59d8a5ec863c822ae31a14",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1388,
      "row_id": "LB-M1-rest",
      "before_sha256": "c216636c73bbfb86d9ae12f452e2074f85381c3724911f9d81b4320547ad80bf",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1389,
      "row_id": "LB-M2-1",
      "before_sha256": "3663621723e03ceefa44f0f6d0a7f0a57361cea0b81cd481ad3e639a4b8f2389",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1390,
      "row_id": "LB-M2-2",
      "before_sha256": "65b8b2fe1354a3cf3bfb070cbcb2e132f86f62816743721b5d3ef295ff4b5d9f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1391,
      "row_id": "LB-M2-3",
      "before_sha256": "8390b67c72d16972632b21f58458329b7085180b2762ee8235246ebc0110af09",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1392,
      "row_id": "LB-M2-4",
      "before_sha256": "83851b0f54ce203693e5b8369439c6e02f205bc4d08e3ab7de10e0ab507a4a62",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1393,
      "row_id": "LB-M2-5",
      "before_sha256": "922b4d312bc3c6c5fac9cc00cf972281f2541c9f0df13ffc103f7730718c5c99",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1394,
      "row_id": "LB-M3",
      "before_sha256": "09cc49318f22389b78c1e342b4c9f994eb284e8c14953cbacd3142a5eec9e1ff",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1395,
      "row_id": "LB-M4",
      "before_sha256": "25c01e32051220db202dcaadc4239bf7c35f23dd246921c95a521d5b5f374005",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1396,
      "row_id": "LB-M5",
      "before_sha256": "da9e85716c2cf34d69748e450e2c132cfb85fd02ad58376e3a01fa766e9bbdb5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1397,
      "row_id": "LB-M6-M1",
      "before_sha256": "2a701167bbbd162ae90032533ab81f4347fde60707ebc88d6030c317ca0e152a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1398,
      "row_id": "LB-M6-M2",
      "before_sha256": "4fe03ec13da6f101f92eeb75a478b6c394d755a36022bccd9b9e9d7852bf11c5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1399,
      "row_id": "LB-M6-M3",
      "before_sha256": "7b0394ee0daf6e97dfd13525d88a90ab0522605393935e76ec45ccb585d1c60d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1400,
      "row_id": "LB-M7",
      "before_sha256": "abee3bfc4fe3cc49814be3f49dcf8b6880064b73213ddd1ab4a96fa366738400",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/language-breadth.md",
      "line": 1401,
      "row_id": "LB-M8",
      "before_sha256": "5f63b3bc7cf93c354906f1c4c7a25302b8a23867a7370ebd1ae35d8eb1f75b4a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 23,
      "row_id": "LIB-12",
      "before_sha256": "a1505b24b9d20b071b47f535452066773f844e46fe8773d6b96452634e13a7d1",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 24,
      "row_id": "LIB-11",
      "before_sha256": "cfb2bb59f60b78ae9190750e3585c395018ceac488bb1740374728c5bbd5d7c4",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 25,
      "row_id": "LP-1",
      "before_sha256": "61e54fe3c6650eb67c6858c93849111b3c13820360b2c481e38be93cbb165f26",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 33,
      "row_id": "LP-2",
      "before_sha256": "14e4d09194a65bdf2651abbd4692a97f51286e13c39d60c7278ab25f52f2a4b7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 39,
      "row_id": "LIB-9",
      "before_sha256": "db7b19bf1ab235efc2ca190078062ee16a13fa4b628006f1cbdd2dfc31464b77",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 40,
      "row_id": "LP-3",
      "before_sha256": "e208f0dfb1e51da1ff16630cfd1035c4940c16bca593dcf80f54b08d116c5c26",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 48,
      "row_id": "LIB-7",
      "before_sha256": "c54eed7a54728cbd5ec8b73eabdfc040186f056866490c01f9cbfbc80fe913fc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 49,
      "row_id": "LP-4",
      "before_sha256": "3f22168c51926a42c403e22c29872f546ab5af61aa39da815626db1775fc7970",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 55,
      "row_id": "RL-M0",
      "before_sha256": "f5480d651a23d1a79a619008830a54cca98e72a40504775b75dcbc7bb572c935",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 57,
      "row_id": "RL-5",
      "before_sha256": "704e145b7c54d34f7a5fc121e81289926e672c2eaab051c5eb06be98ce347c32",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 58,
      "row_id": "RL-M2",
      "before_sha256": "b870354046710b44327056c90b705539b35ca0013b4e1d2b96d3ad1e95f6fe5c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 60,
      "row_id": "RL-M1",
      "before_sha256": "e5d4d5543ddbef7e1ca52db84351026da7e0960bdd3952cac0daff4823abde66",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 62,
      "row_id": "LP-5",
      "before_sha256": "903a6a0b14f3973b810b6d5a7b63bdc695e17dc4ebd64169fdcf41177eee2fb3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 68,
      "row_id": "LIB-13",
      "before_sha256": "b2265061ae3d11d60c54421d3f1241cdbf909970e28cf6cc83702aaade1bec4b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 69,
      "row_id": "LIB-14",
      "before_sha256": "f52a047cddecec62bb3b53daa2e11705c3ac4c038e0be69487b399fec33c6411",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 70,
      "row_id": "LP-6",
      "before_sha256": "381c28de29c97acb64a7b06012b43dd84cec8b3ebba0e593ddf859591a7fd66d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/library-roadmap.md",
      "line": 78,
      "row_id": "LIB-10",
      "before_sha256": "51741fffdb8cb40834bd408dd5f9a174e39ef91bdcf870b891d0123056c08294",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/multi-machine.md",
      "line": 11,
      "row_id": "MM-M1",
      "before_sha256": "f7f8118c71e356477c69ee1d91886b7716f289f6cff94ce27efe8f61d2ef24e7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/observatory-console.md",
      "line": 85,
      "row_id": "OC-49",
      "before_sha256": "fe10edcd8e36ad542126422bcb5873395e3ccac1a866ea7f80e8ca7a3b0de956",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/observatory-console.md",
      "line": 86,
      "row_id": "OC-50",
      "before_sha256": "b1bd563800041bad6900d6ceefed936f150e64f80d847a22d9b891ad2f1335c4",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/observatory-console.md",
      "line": 87,
      "row_id": "OC-51",
      "before_sha256": "7fc4dda286c6da61a9adb98440ac37b2a41605cd90e7982f9388f8f545203cdf",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/observatory-console.md",
      "line": 88,
      "row_id": "OC-52",
      "before_sha256": "3d2d0452db9d60f53e967dc66b495f4b373fc596b344c502752ac262553c4bbc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/observatory-console.md",
      "line": 89,
      "row_id": "OC-53",
      "before_sha256": "848210f9f170848288c40ebd4e5e746e949b40ed8516409e26b8886569a242a7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/observatory-console.md",
      "line": 103,
      "row_id": "UX-M1",
      "before_sha256": "d8c00797c272e38f3b1a0dd1de8fd0eda363d73ff80f926ffd873fc5048a2363",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/observatory-console.md",
      "line": 104,
      "row_id": "UX-M2",
      "before_sha256": "dd21ba408b75e1f8975f98de9ef6ada3a52dff7a535889901633e9c8c1b7d67e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/observatory-console.md",
      "line": 105,
      "row_id": "UX-M4",
      "before_sha256": "cb9a633741289a7a04298c0f3780e309abf951e80b1a62d5f8dcc00359d275ce",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 28,
      "row_id": "PLT-0",
      "before_sha256": "20bd907d550feccfdab89a3d945ae1599ace195ee38ba00f356202a18701cded",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 29,
      "row_id": "PLT-G",
      "before_sha256": "62ac3d96fbc7ba3994fe8528384729ed86d2fb71a17e858f923b761f161d78bd",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 30,
      "row_id": "PLT-1",
      "before_sha256": "d6c6585ff5a79bf20e2b4752d2c814673e17520f80d90953cfd04f0b3ff37e5b",
      "gate": false,
      "lane": "cloud"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 31,
      "row_id": "PLT-14",
      "before_sha256": "118dc8057a1b77942cd77124047dbcf769c9dd3921fafeb265138bcc3e402fdd",
      "gate": false,
      "lane": "cloud"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 32,
      "row_id": "PLT-L0",
      "before_sha256": "5b7ca4ba69b2f9cf9ef7ad5ce4e83900f6b563343c58793a099682d962a55ef7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 33,
      "row_id": "PLT-M0",
      "before_sha256": "6f942a8c0e611923b6fc4c4506675353190594cc1415c4ac76471470fc8166c8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 34,
      "row_id": "PLT-M1",
      "before_sha256": "1b0b71643560a4f7ae77a5d5387e738d6f3c58db861f51e043bb3efb3afb724f",
      "gate": true,
      "lane": "codex"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 35,
      "row_id": "PLT-5",
      "before_sha256": "daaa098678c243731eb7eeae4fd8ce96b0e37cd5f89038de4a5f14150f7deaba",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 36,
      "row_id": "PLT-M2",
      "before_sha256": "02543655890373a530aad055e9a0550df623cc07b055503daf5057ee35c5c263",
      "gate": true,
      "lane": "codex"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 38,
      "row_id": "PLT-10",
      "before_sha256": "7006f9db12b8e1e649ea8f141f41f47eb22a040a5c7c9f43ea6f8aa33f56261f",
      "gate": false,
      "lane": "cloud"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 41,
      "row_id": "PLT-7",
      "before_sha256": "a8670217f1bf7949ae4d716beeecaef46c0dcb1f99d3b8feeb718b1f0a3f48e8",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 42,
      "row_id": "PLT-4",
      "before_sha256": "72ada6f589e9bda293a2957bb793814950b37a93cbebe40007cfd342616c3cd2",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 43,
      "row_id": "PLT-6",
      "before_sha256": "d9b2aecc28b1d0a2181daee85970f3f33f569cf410b0b39a65304fbb4e38070c",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 44,
      "row_id": "PLT-15",
      "before_sha256": "0f8a44c8789ab0155de03ab05c3641512364932ccde0aff0a190216b18ee28ed",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 45,
      "row_id": "PLT-12",
      "before_sha256": "0166064f436187c6e4453295572e6c39a4d912eb2973f6503f63eb0068fc664a",
      "gate": false,
      "lane": "cloud"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 46,
      "row_id": "PLT-17",
      "before_sha256": "5bc25c4f4a4a8b23e48b48acc62c115b1dae09cf12dfd5f68443d4efa1728969",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 47,
      "row_id": "PLT-11",
      "before_sha256": "d29bb0dfa11eae68528557e472450b71ec37ef8335b7359c22df8e2170ad1830",
      "gate": false,
      "lane": "cloud"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 48,
      "row_id": "lab-platform-skills",
      "before_sha256": "6cfeb36ef10b42dda13631cba5833e3b8470245a91b62853bacea22566e44f79",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 49,
      "row_id": "PLT-M3",
      "before_sha256": "83fabc1cea6a20e6c5295d925f847011beaea90d0a8ac408eef71c1d82b146f7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 53,
      "row_id": "PLT-M4",
      "before_sha256": "764fb49de549d321af1c72bae8915494f732a91f931c1c91fb4b5220387d44b9",
      "gate": true,
      "lane": "owner"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 54,
      "row_id": "PLT-M5",
      "before_sha256": "c818dae9f94d1ab97ad56052d2b49e2f5e6ae44f1056fd562527a436884236c1",
      "gate": true,
      "lane": "api"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 60,
      "row_id": "RVU-1",
      "before_sha256": "c3e92ab879233186d4752d5e6f250b9c62c3991c87768f9f53e975b7b17e8398",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 61,
      "row_id": "RVU-2",
      "before_sha256": "12c201c877c3a56df6d4b53823608b776282877498916952f95b3fe41f80acc0",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 62,
      "row_id": "RVU-3",
      "before_sha256": "cd42dfa1cdcd81bd78554754dfa827bd42cab568823e36a2dcabfdfc4ece44fc",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 63,
      "row_id": "RVU-4",
      "before_sha256": "16c411abc6eb96b57f81047f5cbf9e55a894cc10e182172c36090e50ea80fa6f",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 64,
      "row_id": "RVU-5",
      "before_sha256": "ab53213db7881fec090c0a0a6f67a5211ed913a9b45053af3e8cabd0359473ce",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 65,
      "row_id": "RVU-6",
      "before_sha256": "6f9e4c878db442a3237641d2ec05d5f474373638599f3f5b283a2e6a0795092e",
      "gate": false,
      "lane": "cloud"
    },
    {
      "path": "docs/plans/platform-engineering.md",
      "line": 66,
      "row_id": "RVU-7",
      "before_sha256": "28def778fd3c471833809b9631fed87f59ed497352b793c88133913852e8bfeb",
      "gate": false,
      "lane": "cloud"
    },
    {
      "path": "docs/plans/portable-receipts.md",
      "line": 23,
      "row_id": "EVX-V1",
      "before_sha256": "af6a1af700a4af37a2e9ed88db0dca628c15cec9637e574e536eac7e7518a3e3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/portable-receipts.md",
      "line": 33,
      "row_id": "EVX-K1",
      "before_sha256": "a7685fdd03d5e764ba7bf2080923b5029bab99b54289321746443adfacd7c765",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/portable-receipts.md",
      "line": 45,
      "row_id": "EVX-M1",
      "before_sha256": "8e37be694f73bd2950f8d09e706e933bdc2856f00aa2743cacd45855270f5a45",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/portable-receipts.md",
      "line": 49,
      "row_id": "EVX-G3",
      "before_sha256": "bf391225ac73d05bd537d9953b8e7b2090bb6c9f54a838b25321e8a7cdc36ac6",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/portable-receipts.md",
      "line": 58,
      "row_id": "EVX-M3",
      "before_sha256": "5535fcdee5a42e69b49569427aadf5b69a2554e29cea000a6ecdb25427a9e010",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/portable-receipts.md",
      "line": 66,
      "row_id": "EVX-M2",
      "before_sha256": "ee92ab436580240cb65668e93684e3e748ebc3f0a825b6a01214a5e871f7a215",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/progress-and-economics.md",
      "line": 79,
      "row_id": "PE-M1",
      "before_sha256": "040f1c2fc70c0b71a83de8ed52afec6868689ecfdba48437f65af0fb7ad666ee",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/progress-and-economics.md",
      "line": 80,
      "row_id": "PE-M2",
      "before_sha256": "510a8d90f3194da7e0084dd964cfe23fa08a5f715b4f70f302f364ed0976e190",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/progress-and-economics.md",
      "line": 81,
      "row_id": "PE-M3",
      "before_sha256": "f24ca7b74d1ca966942b043e3a579e9a883d658249a6e35d04a64a18423b9cb3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/progress-and-economics.md",
      "line": 82,
      "row_id": "PE-M4",
      "before_sha256": "39cd8d4e00d765cb31307b7cdd1a21db6214acc1b4697575f0995b0a1e7a77b1",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/progress-and-economics.md",
      "line": 83,
      "row_id": "PE-M5",
      "before_sha256": "f5bcd3b5ddac60f597005f07f66b93a2c80342b55bde9e91fbe70885d43592d1",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 24,
      "row_id": "PU-D1",
      "before_sha256": "ef88410129a615e7b46025b8538843420641efbdf5e8138b112ed9b9c999e2d7",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 25,
      "row_id": "PU-D2",
      "before_sha256": "3a4cf25d34de82623117e809ceb441b1cc92b9603edb9bd55b60d1c13394bc40",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 26,
      "row_id": "PU-D3",
      "before_sha256": "849a52ef5bfd613c897858344d9137a4c8669b5eece8b38ae64c911737f928a8",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 27,
      "row_id": "PU-D4",
      "before_sha256": "44ae5c3280f7ffabaa8ebc3717dca160ba056290ef713af92a9ca95b2ccc7730",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 28,
      "row_id": "PU-D5",
      "before_sha256": "97e1d27cad249845741b1f63dc290861efa7b0e48134e6c0dc53c25cc83cf430",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 29,
      "row_id": "PU-D6",
      "before_sha256": "26e965f66e85e28dc43ca9e1775737342ea38f70b5cd495120e6ec9984863c9c",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 30,
      "row_id": "PU-D7",
      "before_sha256": "f9531eb13a0ae9850a310b915e6ebeee82c009c496c8871fb43bb954a7592083",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 31,
      "row_id": "PU-D8",
      "before_sha256": "cfaa28229bfc9eb95083faf521b1de3fce892e107029bf79190bcfa5f2e04129",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 32,
      "row_id": "PU-D9",
      "before_sha256": "d9cb16d834c44cde10157f9b69f8f7d261bb6a1b696ce4c0646c4407b76dc6ed",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 33,
      "row_id": "PU-D10",
      "before_sha256": "d6fedba94636a38cdf73e4840c9d096917a044cf5ff12a7dd618b75c56b5c633",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 34,
      "row_id": "PU-D11",
      "before_sha256": "bf041df9ccda29faacec59dbc342966fd06f78dbb0d312b0083cbd9c3bfb8c08",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 35,
      "row_id": "PU-D12",
      "before_sha256": "cbdf9cd8b43d50707ed0072d8cd511cd7e69ad0cff5c30415693c180bf40503a",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 36,
      "row_id": "PU-D13",
      "before_sha256": "be8d427e9dad2d9270514f3d01eca746cc6f51795b92e63c523a45f38279a50b",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 37,
      "row_id": "PU-D14",
      "before_sha256": "562b59272e7acf48d7db606a99adcd4cef827bf13e06fbe693433aa9d5a22ba9",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 38,
      "row_id": "PU-D15",
      "before_sha256": "ee65c4e1638999d509a39890f98c42444bbc2ef6e5ce82bfa096f620634c8f88",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 39,
      "row_id": "PU-D16",
      "before_sha256": "90ba1c222bdefc82c4c70fc12d75bc66dd7ac7e93a9c1f2f93ff19209ab84ae9",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 40,
      "row_id": "PU-D17",
      "before_sha256": "a72f6c670ca1fd9dd95a03ec40e44598a46e50bc57f2721fe2900e1cf9ad7fde",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 41,
      "row_id": "PU-D18",
      "before_sha256": "b4260dd4ccf9fdcbd4fb4638cab12a0ee2a3c5bf6bc370a51d1a8970e6c2f1cf",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 42,
      "row_id": "PU-D19",
      "before_sha256": "10d423e4bfadb3c7a101a1334a04e4bf95c61960bafcc46054d36d8a4249692d",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 46,
      "row_id": "PU-L1",
      "before_sha256": "355f53583c3d2dbbafc33f626ed6c8d89c46712be65aced01afed1e983aa7169",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 47,
      "row_id": "PU-L2",
      "before_sha256": "0d14de53080dce2913c5865f138342991477facc9daacd954ee90f89fa244792",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 48,
      "row_id": "PU-L3",
      "before_sha256": "fe6fe2a4d1a5d4a166957bcba3f3b6ccea5a07ef8aa7fa843375b33f950b0224",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 49,
      "row_id": "PU-L4",
      "before_sha256": "f58396aae58445123e048eba64789c79bcbdc2174d98a93f657733d1457326d1",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 50,
      "row_id": "PU-L5",
      "before_sha256": "1dba9e5ec07944dafe05f8feecffb679db84e5e44177c1510ee3613feffc6f31",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 51,
      "row_id": "PU-L6",
      "before_sha256": "c31996e5dc30a76a4a5ab0e158ca3aed7cb45d22903c582a541cb37426732d68",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 52,
      "row_id": "PU-L7",
      "before_sha256": "567f97f9d3ffa20248c0218173909e6d1baf1de264dfb02f58f6fccc1d778fd8",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 53,
      "row_id": "PU-L8",
      "before_sha256": "31b992c565eff9dcdf34d6efa10ec776c6b8774c8bb263193fe7ad7ed33cb5f9",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 54,
      "row_id": "PU-L9",
      "before_sha256": "3d1389aa47f4b14b930410ea5f4cf490f4f3a05801b64ea40143738d99ed985d",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 58,
      "row_id": "PU-B0",
      "before_sha256": "d12ac829b33a52d06550c60c81c94288975f527f3e0e72448c830c4f29372273",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 59,
      "row_id": "PU-B1",
      "before_sha256": "d9247a5c1b25c029e05c19f5c3db33736ee8ede5e486b3459525c09be86af87c",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 60,
      "row_id": "PU-B2",
      "before_sha256": "672febb84361ba96ca1c153f5fd2afd2382f91938dba48163d245b3f1a8567e5",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 61,
      "row_id": "PU-B3",
      "before_sha256": "c43612513ba5ad5bb4448ff2c5da0c76acc0dcb5a5ae995511beebcf3043cb1c",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 62,
      "row_id": "PU-B4",
      "before_sha256": "18ea08a1ff3272b1ef9a25208225f4250d1bba2584cf2ec2637a510d5a039349",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 66,
      "row_id": "PU-M0",
      "before_sha256": "2139a87bd665135ca8fbbcb4a8183fb306484dbe093f03e71094e86d17c24278",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 67,
      "row_id": "PU-M5",
      "before_sha256": "96cdb302fdce617cb3e40f964efaace24befbfe139de80f868f6417857dca331",
      "gate": true,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 73,
      "row_id": "PU-W1",
      "before_sha256": "709eb67392f7df1b54ca103f73ef7342d3e3e614f3dc50c961f20cb9f258a50a",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 74,
      "row_id": "PU-W2",
      "before_sha256": "c2692eb0b1a6c5b3c3dad314301a0cdf37c0ee0f62b9a08c2546779f7720923f",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 75,
      "row_id": "PU-W3",
      "before_sha256": "eb357907735fbfb0ac51fd989eb8dfcb740aa620225c3b510130088e5098c13d",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 76,
      "row_id": "PU-W4",
      "before_sha256": "69a75efb84170669c30d847aacf59bba428d12829234643b5fbd2137f70f4b42",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 77,
      "row_id": "PU-W5",
      "before_sha256": "f994b46f7c0585236e26c53c2d7fc1f12efb1a530f4633199eed74534fbf4b48",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 81,
      "row_id": "PU-C1",
      "before_sha256": "38d0e551f198fd59d38111593e76f53d480850051c2aae7d2d2c536d07509397",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 82,
      "row_id": "PU-C2",
      "before_sha256": "ec5c79fdbe28c72fc09168b12b717a28c1a0dff224fc3b0604fa5d125c672909",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 83,
      "row_id": "PU-I1",
      "before_sha256": "5b2261f081c3a281960d3ba5bc5b7ef70d2f58b250c4fcb8534f0ae667b307dd",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 84,
      "row_id": "PU-I2",
      "before_sha256": "9accfb2894137b954e80ab5976ba81cd74cdf0be1bbf4457cba5a9b88b7f14b6",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 85,
      "row_id": "PU-U1",
      "before_sha256": "1e035f1bcda13f5017b234cbd7871d7c285548d104c17329092ae1b097bad3cb",
      "gate": false,
      "lane": "cloud"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 86,
      "row_id": "PU-U2",
      "before_sha256": "414b5885179a5810eb364110374df5851cedd7ea1c578ccd29c0caaf6f158426",
      "gate": false,
      "lane": "cloud"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 90,
      "row_id": "PU-M1",
      "before_sha256": "746d322abcc317c207ed1c1e950734c20702c7e40d532c08b67f7df099330987",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 91,
      "row_id": "PU-M4",
      "before_sha256": "67347e5ee0918f5deaac0371114d99462f08c9ec1b0ce02a1ba466f5e696873f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 92,
      "row_id": "PU-M9",
      "before_sha256": "c81d7fbdb1c84b373d3325462eb88b2ba97202dfe90c1616dc6860c1cbe0a31a",
      "gate": true,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 93,
      "row_id": "PU-M10",
      "before_sha256": "e4302dba4891c50e04287d4ed266247e32a71b2f7a1b84cf10b2cf6161a361b0",
      "gate": true,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 94,
      "row_id": "PU-M11",
      "before_sha256": "68db4d7332205faec14f3079a3f3ea12b4c42bbdcb42f68b8d64a4b39911b82e",
      "gate": true,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 95,
      "row_id": "PU-M12",
      "before_sha256": "87d5d5f376ba1e92e744449b66ec744a3459620247a5656b21e387d5e2410e12",
      "gate": true,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 101,
      "row_id": "PU-W6",
      "before_sha256": "3c8540619b00dd2d47c20fd5e347aaa185a5b5a14dcdb040ede320e7c4d7ebfa",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 102,
      "row_id": "PU-W7",
      "before_sha256": "1791db2dbad80e1ac23876d02934eec333dc8124258fc455b28a546171505492",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 103,
      "row_id": "lab-why-card",
      "before_sha256": "ac5993199412bf8527daeeb7e31d20790505ce8dd45658492f9a47307fd5af55",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 107,
      "row_id": "PU-M2",
      "before_sha256": "8128ae4a5d386ee88e2d86e08f21f1bdab2158dbedae95d43b5dbdae4182b68d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 108,
      "row_id": "PU-M3",
      "before_sha256": "66957fd2ee9f35fe8e1826f060ad0a025233560a26a79d0b6adc425805233331",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 112,
      "row_id": "PU-V1",
      "before_sha256": "df5e989a1e7d85b987370e466208228d33d62940029ff6b0b20e711295329c82",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 113,
      "row_id": "lab-reviewer",
      "before_sha256": "1d7add3e160273b90afdbb52566b32638dc4dab00f9493003a5b45609deecea7",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 114,
      "row_id": "PU-M6",
      "before_sha256": "2413266110e091472287aa7bc4f1166c4ded8d3c55fcb3ec5e0cee6b027eb656",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 115,
      "row_id": "PU-M7",
      "before_sha256": "d309c34bf75a1a819f6ad8530bcab3e3a02774ee7372cc5b1ba5da264beae40b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 119,
      "row_id": "PU-R1",
      "before_sha256": "34796902c92b157c453ae602a5290dd6233e30e0c79d87cf3f2350a08e3f4d48",
      "gate": false,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 120,
      "row_id": "PU-M8",
      "before_sha256": "4d532029291b4ad017704f946998d8f8ce6184c654631d2b4eb8de2275948b49",
      "gate": true,
      "lane": "codex"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 124,
      "row_id": "PU-G46",
      "before_sha256": "411dd666032b67f31259ac14597d0e1dd3a02101935754456b8dee875ee6c8cd",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 125,
      "row_id": "PU-G47",
      "before_sha256": "76801017b6507ce7215f67742b8791b040a463b69646aa6afd897d3d75cda676",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/project-understanding.md",
      "line": 126,
      "row_id": "PU-G48",
      "before_sha256": "53a7dba7f38811eab664b75cc1604392979d0496aa398b717d6e0e50771c6e1e",
      "gate": false,
      "lane": "controller"
    },
    {
      "path": "docs/plans/proof-program.md",
      "line": 30,
      "row_id": "BEN-M1",
      "before_sha256": "071bfced1ba05004de6dcac60728bab12b6f4d83f849b5dc4093585a75234cf9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/proof-program.md",
      "line": 34,
      "row_id": "BEN-M2",
      "before_sha256": "96bf69acd3cc1b8ff66f46d5e4acfdb2b43c1e7d6ee13639028ce2b1e892d157",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/proof-program.md",
      "line": 35,
      "row_id": "BEN-M3",
      "before_sha256": "85e5c177cb39a2fbc7d26b2f1b54f5204d221a40a1884ee8508d9566c07bdbc8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/proof-program.md",
      "line": 36,
      "row_id": "BEN-M4",
      "before_sha256": "94b57095f35259a1f4bbd599325716f26e082b8661474a81be6c793881a6bf78",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/proof-program.md",
      "line": 37,
      "row_id": "BEN-M5",
      "before_sha256": "b11985f771607d97a20a5b18c05bb52c9c8ace141d5a716515de86c86cca243a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/proof-program.md",
      "line": 38,
      "row_id": "BEN-M6",
      "before_sha256": "4e82441df6e5cf347fdfaa353d81980eb92b49f05adea1ae545f43a7b35cc7b5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/proof-program.md",
      "line": 44,
      "row_id": "BEN-M7",
      "before_sha256": "e8f2ef2182121af5a094be313ca77e26c727b3e7054d587451c4e9220a3deeb3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/proof-program.md",
      "line": 48,
      "row_id": "BEN-7",
      "before_sha256": "8e69276e9b99d9bfb241138fa599a4bc420c398aba43cfd4ff67c1359e56318c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/proof-program.md",
      "line": 49,
      "row_id": "BEN-8",
      "before_sha256": "0d403e6c69c6b165bac9a6072194d85366e604b0eb13fedba7506e341a0e84a6",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/proof-program.md",
      "line": 51,
      "row_id": "BEN-M8",
      "before_sha256": "e6b71ff422a666a0b7790463e303d94145bb6945f1f77b18ad3566f96717d5cc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/proof-program.md",
      "line": 52,
      "row_id": "BEN-M9",
      "before_sha256": "29946c65104fa4f2ef4d9edbff601fa8f3bfc6af94c08aea4cc64432c62db621",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/proof-program.md",
      "line": 56,
      "row_id": "BEN-10",
      "before_sha256": "a98433073d9b683b00ca46f9423264c8ca988eec482e7129ab7167d22f1af925",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/proof-program.md",
      "line": 57,
      "row_id": "BEN-11",
      "before_sha256": "7462462e5d7082520676d0afd683bcfa2a4b71715625766037065723cf3dc2d6",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/reach.md",
      "line": 57,
      "row_id": "RH-M1",
      "before_sha256": "260b6c8f82ed1295fd80ef7caea8007918d83d4de6df5d2e3f1bc2536e6eb763",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/reach.md",
      "line": 58,
      "row_id": "RH-M2",
      "before_sha256": "e746099c53d513bb24103ddb7489a23f6bd9233e2b8e8435243905a516c8e3bc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/reach.md",
      "line": 59,
      "row_id": "RH-M3",
      "before_sha256": "a665b5b06c168deaa18d59e3cd2dcb882ea0c0ba98ec5975920c937268ae2664",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/reach.md",
      "line": 60,
      "row_id": "RH-M4",
      "before_sha256": "7dfb1410d90d9106745e165abac8a5be2716c1a91696070d0ee90c50bb028a4a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/reach.md",
      "line": 61,
      "row_id": "RH-M5",
      "before_sha256": "a1daa42dcd2c1d1cff526fa332facde5933d5fb679b760fd8367a09ae82fdaaf",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 45,
      "row_id": "RO-M0",
      "before_sha256": "78f9f44af71a2a6de4f96d8aaac0febe551f657a0c31ab4658d07efa6df355b7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 51,
      "row_id": "RO-M1",
      "before_sha256": "7e34a3034ff7db7d4f18d73f7cc2aa2da24933a8c77071d844339b2bea1b2579",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 68,
      "row_id": "RO-M2",
      "before_sha256": "f308bcf56be86dc09597d774c1fc8df8055b862d560de2b62abcf03274e21812",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 69,
      "row_id": "RO-M3",
      "before_sha256": "b4bf68411e287474e91a1aa16a73b912749b5ac896448fb946a241261cc48ed0",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 87,
      "row_id": "RO-M7",
      "before_sha256": "30114c099f59e200b0fdbd2205cddae208281007703a93b1d96278d3abe40a9c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 88,
      "row_id": "RO-M4",
      "before_sha256": "cf4c15571e7617886db54958b91277ffddbb77a1c018f84cd9f5a82a8bd821a8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 108,
      "row_id": "RO-M5",
      "before_sha256": "d9322b5ee45ca234787e32c2029bd6294f617b201d4581f1e23e10d0d4bb340a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 109,
      "row_id": "RO-M6",
      "before_sha256": "e3c644f0addf6762fc7d45c5ddbedeb70677a48fb417ba533a486c463582b8e8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 119,
      "row_id": "RJ-M1",
      "before_sha256": "c18f0d13c31f0829f5f651a565531e700b5488ed247c3cb3da67c86fda9137ee",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 123,
      "row_id": "RJ-M2",
      "before_sha256": "df99a1ad44fd3c59d95d9708b135d7c8e805e2856679ac43107576a875bf380e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 149,
      "row_id": "RO-V1",
      "before_sha256": "17a2121522068b03394f9c83ce02e8961f8d65749099eb5975a37463efc20284",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 153,
      "row_id": "RO-M8",
      "before_sha256": "40b3108ba74681d0f4bba51c45a26cb7723fd330fdc319d0ed8fa38dab44b725",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 155,
      "row_id": "RO-M9",
      "before_sha256": "234351803b81e978223fffcd568aa1f33409ec10378d561d8c4c5b0df4658a9e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 157,
      "row_id": "RO-M10",
      "before_sha256": "f70310bcb5e4bd510843428f28afc20ecc4b9c184b7e9a8082c0aecb23849738",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 166,
      "row_id": "RO-M11",
      "before_sha256": "dfbfcd50e96078adf2609fef0c38d4c4f3c365b96aafd04dcc4bdf4300be684f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 168,
      "row_id": "RO-M12",
      "before_sha256": "3e188d67256ab8471d6bad76dede5f64ea1711600164c436d5906b3ebccb3b4d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 169,
      "row_id": "RO-M13",
      "before_sha256": "c9b1bd5f6c86c37baf4a7b22a7728789047012398bc947b19b846f4bdcaef2b9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 171,
      "row_id": "RO-M14",
      "before_sha256": "7388128ab691702f49e125080d8feb406e7d0280b5f7b0a1066ed1bb8b7e4bb5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 173,
      "row_id": "RO-M15",
      "before_sha256": "deed271d5b47d55b1d05720f4986fcb922c8e0b089e5e8682dd727b9d07af527",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 174,
      "row_id": "RO-V2",
      "before_sha256": "feddf01de7a9dc40d36b07401e8f738f8495d7e4f827b79c41590ba65854c9ef",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 175,
      "row_id": "RO-M16",
      "before_sha256": "ed1c1f7a6891cee96035170578569c0fcc26c1aeeff8185a36973a1b4108fe67",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 177,
      "row_id": "RO-M17",
      "before_sha256": "20e48003a22559790b2241e11b15d2a9fb4527e3da613d74f53a47be9c6e7b9f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 179,
      "row_id": "RO-M18",
      "before_sha256": "60f78034204f0e09bf3f819774ad01f726bcbb6144ba8ba4675c51360099e78c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 188,
      "row_id": "RO-M19",
      "before_sha256": "6d426b782484a201986f1df896297a781ad52f0251d40c8b8892105f4d5a9de2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/readouts.md",
      "line": 190,
      "row_id": "RO-M20",
      "before_sha256": "a3fd188be5dc04d250182922ff3f0e181d326cccb095ea082d820f43b646d028",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 78,
      "row_id": "ENG-1",
      "before_sha256": "09df0b68b102fdcf0819be5ca5e5ee374ad8c4a560cbce4d4bfcbc9022be036a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 79,
      "row_id": "ENG-2",
      "before_sha256": "7ef10dad6c19e55e8afb64cb87e71aa3c394d417269f1be75470c7167b88c5b8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 80,
      "row_id": "ENG-11",
      "before_sha256": "a0959085b0ba3dda2a0d79d6065bc04e7da2299662e4398fd16f81e36d581c1c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 81,
      "row_id": "ENG-M2",
      "before_sha256": "64c32e2f8fba56e819b4ab25563dd581a1109876228c3353c7270813052bb6b9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 83,
      "row_id": "ENG-4",
      "before_sha256": "d971108488db2d1e0c90b851d35db0acb5d6d559825573177748d9ea5c05aa19",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 84,
      "row_id": "ENG-M3",
      "before_sha256": "1b73002863df72cd8118ba633017f76168af9c3517004d86b8595e7aa2294300",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 85,
      "row_id": "ENG-5",
      "before_sha256": "ae0268341add393d43dadb6f93504ab55ed3387989cc69942fe3def562af2007",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 86,
      "row_id": "ENG-6",
      "before_sha256": "e3d1cde460031e25303c1287e764655b9e8b7e878ef398e6d6bbd03b0c29291c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 91,
      "row_id": "ENG-M1",
      "before_sha256": "75548ac9ce340b77fb945f1661cb3a524fbeb48d0fd5926b8767db94390cac66",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 141,
      "row_id": "CHG-1",
      "before_sha256": "a3610882984d08c1fffce0c32e0f80d31ded6300c08d0ebaf0f3ad6966260ebf",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 147,
      "row_id": "CHG-M1",
      "before_sha256": "076e3f502bb74493c6cbbd6c27dd176775b19ae721bf1f89f789b534f5063737",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 149,
      "row_id": "CHG-M2",
      "before_sha256": "7e05c6010a94d97b658cdcbab4a8c54717698f28f70f149cb6fbeb832ba1a399",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 151,
      "row_id": "CHG-9",
      "before_sha256": "9b90fab3149ebecb1aa9378c7fbf4de143b4f69e0099d31dae1d67d614ad1643",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 152,
      "row_id": "CHG-10",
      "before_sha256": "5f9ef5c27a45d5516b303e78c50355cbbebe36fa1eabb30724a655f8b2befdcb",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 174,
      "row_id": "LIB-1",
      "before_sha256": "d1249ba1e6e5809e32ccdd8a51ca0a25357d81d31929efecb6a20fc18e8efb83",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 216,
      "row_id": "FCT-7",
      "before_sha256": "b4c0a3c147bd08416e939843c97273800f317d865667ecd259b52f2b2e545cb3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 219,
      "row_id": "FCT-10",
      "before_sha256": "13699cd5af9384d4fcbd6f47aabe341d5a39bd0d136bc575f15e817d6b332eea",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 246,
      "row_id": "UND-M3",
      "before_sha256": "8f4426c4c19c38c59ea28728eb9fe66451c11d5db8143a98f5c8c245503f89d9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 253,
      "row_id": "UND-M1",
      "before_sha256": "d873051b3dc73d0b6437f571ca691097e1cfc152feaf21dc814280a937420e21",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 254,
      "row_id": "UND-M2",
      "before_sha256": "4a07009f921025ac679e2b5611a387a4677f3722fae573920f363514ee73b153",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 267,
      "row_id": "WIR-4",
      "before_sha256": "d1668eb5aa8f0aa449b26b2e6f17b2939b82c733222b703d8967d5229de0b978",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 269,
      "row_id": "WIR-6",
      "before_sha256": "66c443f86a7dbbd7cc9dbcb32c085081c2d69ba8ad79f55359cfbc3d1bd135c3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 272,
      "row_id": "WIR-8",
      "before_sha256": "f67abed9efebe493b4720bfe733ef3346d62f2582a96df6b989e33b5f5fde159",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 299,
      "row_id": "CUT-3",
      "before_sha256": "fe2d746982f8278ec1ce62018e302968e261b9bd1938134054e228071bda1f1b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 301,
      "row_id": "CUT-5",
      "before_sha256": "143f443017f67b25835f31497d88e53088102ff5474fe8ee7d0c9967e1295e41",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 316,
      "row_id": "INB-3",
      "before_sha256": "dba25fc08210951910f0fcbb5189a7c611ada284fa58c59aa790c63f9d0ed794",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 330,
      "row_id": "TUN-3",
      "before_sha256": "82829dc0f33cf6bfe2a5756d0d571979f0d5fcfc6dcebc58f544364d6e5c1a29",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/realignment.md",
      "line": 331,
      "row_id": "TUN-4",
      "before_sha256": "69ab41f643638f02edcd12da32273a05b01a68d08430e1dece01ebbb08e142ea",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/receipts.md",
      "line": 79,
      "row_id": "RC-43",
      "before_sha256": "3f50ac7760ff0b9f25b5db7652db51c02e414a0b4697d307bf83abef19557715",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/receipts.md",
      "line": 80,
      "row_id": "RC-44",
      "before_sha256": "ed6c22e1f4dc0483ac15704c72fb6369a5c9f899ab957a80c49ed8719ff87c83",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/receipts.md",
      "line": 81,
      "row_id": "RC-45",
      "before_sha256": "7ccfc380ef98c51114dd1ffcc92991e82041f8043f0e61a991aaa24087dec1ce",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/receipts.md",
      "line": 82,
      "row_id": "RC-46",
      "before_sha256": "18b9bcc86016e738df1c0c740718b2fc2af864e6cea4478aa846ad407f7b7d45",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/receipts.md",
      "line": 83,
      "row_id": "RC-47",
      "before_sha256": "6e689ae5d4883b865e40571fe8fbd9697befcc6781cd3a86a86d16dfd3ba2f65",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/receipts.md",
      "line": 84,
      "row_id": "RC-48",
      "before_sha256": "9ff6478e7d520a3edb0f5ac25b2ae9fd84e55fbfc294bc831340d2e1f1853cdd",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/receipts.md",
      "line": 85,
      "row_id": "RC-49",
      "before_sha256": "0bbea488d2402fcc91960ffac047cd39cee5489a8aaab925b547c4f53c383162",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/receipts.md",
      "line": 87,
      "row_id": "RC-51",
      "before_sha256": "454639adef0faa2a46433902f8265a5c5ed249b28d226e5980e945b31f0932c5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 24,
      "row_id": "CV-M1",
      "before_sha256": "b48075269cd19cb3b6eae2ff129e4347d44a7bfcd65897900200595f9c3b7d26",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 25,
      "row_id": "CV-M2",
      "before_sha256": "a3c7b35d425e9d796c80abd300e2c974362ecd7cad0edf4fc47833615e7eff7a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 26,
      "row_id": "CV-M3",
      "before_sha256": "666399d0196c794c0e0d1f863a5842de788b1754b200e0668680aa089cda2f17",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 42,
      "row_id": "CV-M4",
      "before_sha256": "22a1dd081ed6c6172085c04c1ac3f6154f6a828a22ecc69d0fcd1b8506245465",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 52,
      "row_id": "CV-M5",
      "before_sha256": "e4f1449ad73481223a9c4ab7f0f36e62996580a286062537177a802efef88043",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 59,
      "row_id": "CV-M6",
      "before_sha256": "404f64bb145019c154154d657137b91fb6db07153c6b781e1801209e16d56d66",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 66,
      "row_id": "CV-M7",
      "before_sha256": "bdf51a10c55d053ff63a5a4b793c87b7f2d4074f908d2e730fcf50773a9147aa",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 72,
      "row_id": "CV-M8",
      "before_sha256": "9f96260fe32a3c183579ce7bf4f310b55d61b5d10728b05d453b20b61d27404e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 73,
      "row_id": "CV-M9",
      "before_sha256": "ab1ef40f459226d74502d3bb219a7e2870e9512ef655048d271a719f1717f117",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 74,
      "row_id": "CV-M10",
      "before_sha256": "6925103accf15183eb33d7d50f13dfc0ab6c7fa75de3ce879837bd3269d8fdd2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 79,
      "row_id": "CV-M11",
      "before_sha256": "68e3bb511af4aefc6c01b902014d1a3a73c24698fc92ecf2a8f2231f62212549",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 83,
      "row_id": "CV-43",
      "before_sha256": "eca29db4cd1aef92754110283c3946d001da1ba7f928676e365168acdc987da7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 84,
      "row_id": "CV-44",
      "before_sha256": "4aba73de35f2f1210645253b4e5947126f5ead6d01f1d02a91bc6c34c171b307",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 97,
      "row_id": "CV-66",
      "before_sha256": "4a677f7f69a0989f9d1a0948dbd89a8bc2565e805fc0304be68f0a55a9c0150e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 98,
      "row_id": "CV-M12",
      "before_sha256": "3a079689dc7cbecc70b883ea471bf3fd95633cf6bb226fdd4f2029a90231df32",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 108,
      "row_id": "CV-73",
      "before_sha256": "168de5af4ac9909018ecca226aa454bb1f36015788c9965df4baf5ddc88d53a2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 109,
      "row_id": "CV-M13",
      "before_sha256": "e0766d77c6508d030cb6416c593c8626b7862427e5f1bbaacc79dd5f3800c79e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/refactor-conservation.md",
      "line": 110,
      "row_id": "CV-M14",
      "before_sha256": "3275b7d817d67b0f673bc49530860b4c4bc72cbf6fd479be0df0add24dc5e187",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/repository-facts.md",
      "line": 25,
      "row_id": "RF-M1",
      "before_sha256": "3d806fe052e33c98cc85358af799b84e9d61e59ba36ca05197dbe427c576eaea",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/repository-facts.md",
      "line": 26,
      "row_id": "RF-M8",
      "before_sha256": "9c4177869bce126ed5036ee7ff88916b55ff027c028b16ec4fb573bf3fe416e6",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/repository-facts.md",
      "line": 30,
      "row_id": "RF-M6",
      "before_sha256": "6a04547017212770b1edb68a4fe5b70afc34e6c85599be248dbb864dfd8babc7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/repository-facts.md",
      "line": 37,
      "row_id": "RF-M2",
      "before_sha256": "eb09f1cd9498bf68e588e0591d9d8d99195ce1be1e49ffaeaf6353975eb503e8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/repository-facts.md",
      "line": 38,
      "row_id": "RF-M7",
      "before_sha256": "1c36db70e78fcddd2844df52fe3c46a8236d6405ac969db4a6b194a79670f4f9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/repository-facts.md",
      "line": 45,
      "row_id": "RF-M3",
      "before_sha256": "7d4e3525802809edb70c1b8aa35cb9a05ba94635338e9e8c96bff8e9e8ed7a1c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/repository-facts.md",
      "line": 53,
      "row_id": "RF-M4",
      "before_sha256": "bc0d424fd2b1c6707542a11c7771081bdb4285ad7861356c01deb52df389abe0",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/repository-facts.md",
      "line": 54,
      "row_id": "RF-M5",
      "before_sha256": "ea6a4ff8c25d633c8fcf2435ab4033bae8d662b7e993eea4a8476ab959a77d05",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 83,
      "row_id": "RT-28",
      "before_sha256": "518ae0b3b10e288dedf767a0de60f112307c3086f0e51a047d54487abcb086ea",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 306,
      "row_id": "RT-M0",
      "before_sha256": "0082c90887e963770bb1eb5bbb3c4b886f863161fbb1b0ac2fd7634386dedd8c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 307,
      "row_id": "RT-M1",
      "before_sha256": "05abbc12a090040957211ca72140721002e8c061992af00a478c89d6a6e4a664",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 308,
      "row_id": "RT-M2",
      "before_sha256": "8ee6dd40ec19b6a7024552e14054925a8924a8bbb8f7804905c6a108ee8b749d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 309,
      "row_id": "RT-M3",
      "before_sha256": "c602d4a98a6c6d9c2d91f60a8d24083dd8ce5d06fcd11fb8ba3ac192c73e8388",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 310,
      "row_id": "RT-M4",
      "before_sha256": "3b83caf848ea20b3c138c69ea3259bb1ac9b68063fe30bbf14ce1c4aab3b346e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 311,
      "row_id": "RT-M5",
      "before_sha256": "8723e982a3bcaa807ba536bf2f6312b54b3ba4bba25d84b624f5c414dcf6355c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 312,
      "row_id": "RT-M6",
      "before_sha256": "e99ed1e6c180581c42803b47d90760bc333f8daf109dc5e830f9820d13cd5cbf",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 313,
      "row_id": "RT-M7",
      "before_sha256": "163e94a86307440c61d6afc2fe679216f063526ddba37d6aefea5a0ae3e94dd6",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 314,
      "row_id": "RT-M8",
      "before_sha256": "183105b734c36de18a45c855654323e50c2cc5ea6c70aa793193f6ec596af73c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 315,
      "row_id": "RT-M9",
      "before_sha256": "d8b38533f023472e8eb50605b5b1c652dd7064c06af2f7506c410707f24a9475",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 316,
      "row_id": "RT-M10",
      "before_sha256": "dba37eaab974c755e262f17fc89c4cd684f42b49d38c724e2976456cf6043221",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 317,
      "row_id": "RT-M11",
      "before_sha256": "e0c37c8966660ba1711a64c178cf61008b7e38a2e3ca34857a7116dbe64c8ae7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 318,
      "row_id": "RT-M12",
      "before_sha256": "37d3a253a5d9a626577b8e764c706eea2a16367b74b3edfbe738d36575a91eeb",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 319,
      "row_id": "RT-M13",
      "before_sha256": "9a0262caf21add4c8095e7c93fb9e164b335771dd096f948d09b07b305c48fd8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 320,
      "row_id": "RT-M14",
      "before_sha256": "a125d20998684ac9a9d6c8f6da50c8a8d39de1e1d6e711f0ffb73b57f4462559",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/responsible-tenant.md",
      "line": 321,
      "row_id": "RT-M15",
      "before_sha256": "492ed4b8102dd668db2bfd085e0ca7f2333e3398674bbda574186c6dd575aabc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/rule-selection.md",
      "line": 16,
      "row_id": "RS-M1",
      "before_sha256": "d8966b219488e5e1f59d65fca243b90a06cefedd4de11e4795c20ae0da57b3cd",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/rule-selection.md",
      "line": 22,
      "row_id": "RS-M2",
      "before_sha256": "d9d746bbefee584e95b1057f6bb94389a0997fab3d1527f42c605dbdd2342bc5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/rule-selection.md",
      "line": 23,
      "row_id": "RS-M3",
      "before_sha256": "679f1645bf5a95a723a80ecf56755be7f76da0bc383038fef7b987db86210a48",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/rule-selection.md",
      "line": 24,
      "row_id": "RS-M4",
      "before_sha256": "8c4e778b03c5f306df2e7fd3ade419e8be1d043d1c687269ef0f34d030d3821d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/rule-selection.md",
      "line": 26,
      "row_id": "RS-M2b",
      "before_sha256": "5cc7e2e7ec0b6f31ecfff15579887a108984dd582fe7f0ebdb2bb6208c22550b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/rule-selection.md",
      "line": 31,
      "row_id": "RS-M5",
      "before_sha256": "ca3454f760102a230b0545d4c097b0bb23ddae6ca3372928368d72350fe79cf3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/sdd-reconciler-v0.md",
      "line": 32,
      "row_id": "G2-7",
      "before_sha256": "c7f42ff2f617e97e7021c76b60bf68b801fd3f21b917e1e836903f4c853c414c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/served-not-kept.md",
      "line": 19,
      "row_id": "PR-V1",
      "before_sha256": "ff7d14f4bec2a66641fecbd7805399c4d80ac7f580c732787a30a26ce1649077",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/served-not-kept.md",
      "line": 20,
      "row_id": "PR-M0",
      "before_sha256": "311ded3c9f21916461284af4d83e9f178cd6a4b54c5bbfe1888234e88b4416db",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/served-not-kept.md",
      "line": 34,
      "row_id": "PR-8",
      "before_sha256": "67c91909ee72dc0a4823d7afc17bd3c6faf96c590e2043e92411efcc2673018c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/served-not-kept.md",
      "line": 46,
      "row_id": "PR-M1",
      "before_sha256": "62f33521729c27ef4f7e61e38c30f1af6134976cacc617b3f94a0eacb18201e8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/served-not-kept.md",
      "line": 48,
      "row_id": "PR-M2",
      "before_sha256": "e8739d5fccbc694755b10a215fad8cc7602a08591dbafe291b38749fe6130bee",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/spec-interop.md",
      "line": 41,
      "row_id": "SI-M1",
      "before_sha256": "4d159b56dad07d4e503ffb8b08ea5a2e90012107130f1edfe9ef78905d210d68",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/spec-interop.md",
      "line": 42,
      "row_id": "SI-M2",
      "before_sha256": "298f4632c6befffee7bf0f5710a90e4d8686178a57959b48651475423b8d3231",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/spec-interop.md",
      "line": 43,
      "row_id": "SI-M3",
      "before_sha256": "768a01ab6ab4da75650bb1843f78bad89261b81fa925958151bc4faee93be232",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/spec-interop.md",
      "line": 44,
      "row_id": "SI-M4",
      "before_sha256": "a14ffb311a5c546de74630027f56e7752b37719f66d0f4689921315e0f8816c2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/stewardship.md",
      "line": 19,
      "row_id": "SV-M1",
      "before_sha256": "cc52b25be69813fa8fed026d46382586596b19926f4c02a0b6e3c5e581d517fd",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/stewardship.md",
      "line": 26,
      "row_id": "SV-M2",
      "before_sha256": "08bc6cd18f7a81aba11f1cd41ab2da907f729c807fd27b57e3542e73298eb598",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/stewardship.md",
      "line": 32,
      "row_id": "SV-M3",
      "before_sha256": "9a5f81e73c9748df38eddb24e786d4254d3d720bd02baa695501a09d9e0d3caf",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/stewardship.md",
      "line": 33,
      "row_id": "SV-M4",
      "before_sha256": "0c7e9c6cc71bee61c8a75aaf4aee7ddf602db20a0007b398934add9aa2e299b2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/stewardship.md",
      "line": 34,
      "row_id": "SV-V1",
      "before_sha256": "241c240e6be9dfe312d3faf5eeb7773789addbf0c6e5aa67cf6278dd89faf482",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/strengths-bench.md",
      "line": 64,
      "row_id": "SB-37",
      "before_sha256": "424d8195e25ed793d4b4a9dc63ab42897aa6b2d8cb6a612579b5cfa964c7d447",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/strengths-bench.md",
      "line": 65,
      "row_id": "SB-38",
      "before_sha256": "e41e691b8feb4b7063880c07e5869d2372b862462b1ca2d1ed2437863caef007",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/strengths-bench.md",
      "line": 66,
      "row_id": "SB-39",
      "before_sha256": "e82287d2310f942b88eb6269b1ae02ce3400568f8275414be4462afac3f56b78",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/strengths-bench.md",
      "line": 68,
      "row_id": "SB-41",
      "before_sha256": "6767413c16699d4fe9b9cea314ed3a1c70ce3f79ec37a0d6d21884e49fbaf6e4",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/strengths-bench.md",
      "line": 82,
      "row_id": "SB-51",
      "before_sha256": "8450d8549599936dab1376623614ac36d0e80ba0ddc370685fe4e625c5c510d0",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/support-matrix.md",
      "line": 49,
      "row_id": "SM-26",
      "before_sha256": "35431a3de58911d76d03aad54e0c3a8f7ae05b01db7f4cc5f08f54f743575282",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/support-matrix.md",
      "line": 53,
      "row_id": "SM-M1",
      "before_sha256": "3d99a5719a1174060be68395ca8685741932166cc802f72922b4c32a4582ba02",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/support-matrix.md",
      "line": 54,
      "row_id": "SM-M2",
      "before_sha256": "f8f7e287b383e25a8c0a0d9fa04e74f88e24e66e5935e73d58fe4e0e6450500f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/support-matrix.md",
      "line": 55,
      "row_id": "SM-M3",
      "before_sha256": "f6024f575fd650cf6f38773b5e789ebfc187ef3afe6471545b189c2a6e1cd107",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/surface-reckoning.md",
      "line": 64,
      "row_id": "SR-42",
      "before_sha256": "2cc137aa2bb835a8f482197e7df6a4981791ee04338e99690bd26af4352742e5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/surface-reckoning.md",
      "line": 77,
      "row_id": "SR-52",
      "before_sha256": "f5a94fb80f68ddff6cccdfbc36d0022ffee2fe1451f9f79226df277ac13fad79",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/surface-reckoning.md",
      "line": 79,
      "row_id": "SR-54",
      "before_sha256": "89412bc125b605ed9f51b802c16ef556e7b74a50fcd7ab18b418c3817990d112",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/surface-reckoning.md",
      "line": 81,
      "row_id": "SR-56",
      "before_sha256": "70c36548ef51745f6ab229d6e2907f7c66c1ed80f53c12fcb8e10454152f90b7",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/surface-reckoning.md",
      "line": 95,
      "row_id": "SR-64",
      "before_sha256": "0554ccb04dbf6eca4f97ad796409963679666949fcc526f9ebe5bf0ecf70d417",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/surface-reckoning.md",
      "line": 97,
      "row_id": "SR-66",
      "before_sha256": "89c886eb064ef50bd0516bbc6b2733dcdf254816e9734cf1f445a915e12caecc",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/surface-reckoning.md",
      "line": 137,
      "row_id": "SR-M2",
      "before_sha256": "2c9b34a3d6fd2c46643d5eeef87e63c06af23a222a7a930e2267dd367b67c7d2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/surface-reckoning.md",
      "line": 138,
      "row_id": "SR-M3",
      "before_sha256": "41a5a449f0a13e442356883c87b08730b8cace6d8c3ef81f7e8089728352ad97",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/system-graph.md",
      "line": 76,
      "row_id": "SG-47",
      "before_sha256": "278bd7d7cdea57841c08f88bbff0e995693bc308dbe53ac6358a674a25b80d8b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/system-graph.md",
      "line": 88,
      "row_id": "SG-M1",
      "before_sha256": "2ef8ae9ea02de74da85d0b0bf35536de103eba6b2cf766fd7a4bd9130f55fff0",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/system-graph.md",
      "line": 89,
      "row_id": "SG-M2",
      "before_sha256": "caf6426372eb5cca56ac72f404b6c5b4ad4cdaa6db6cbd62a0d85b6e73241024",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/system-graph.md",
      "line": 90,
      "row_id": "SG-M3",
      "before_sha256": "12de3ca3e6089b0094f6c251e075b6935653c8a6559ace164844832ca1f01eb0",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/system-graph.md",
      "line": 91,
      "row_id": "SG-M4",
      "before_sha256": "ef6efc048ffcb320e092d1b7299b5942b4c5c87002e8b6bb32a14158b69c4e96",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/system-graph.md",
      "line": 92,
      "row_id": "SG-M5",
      "before_sha256": "17da572c1e3b2bb656c082b911f5cb53066e79ff4e0e2c8fc5312a0bf96c2217",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/system-graph.md",
      "line": 93,
      "row_id": "SG-M6",
      "before_sha256": "24c0f16184c20b2acfb939d41184870bf5b454c989569c942209f7c80db7a39f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/telemetry-links.md",
      "line": 36,
      "row_id": "TL-16",
      "before_sha256": "1f91a2ab77f0c6642b07fbb8cffcd7501a72dc632192493419e71061895964db",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/telemetry-links.md",
      "line": 49,
      "row_id": "TL-12",
      "before_sha256": "4591466bea4f9955bd324aee0271eef744c2cc80c5ac601c7debc46fe88f13eb",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/telemetry-links.md",
      "line": 50,
      "row_id": "TL-13",
      "before_sha256": "50a35d75f12502d19f9f70bd4aafc2efda3ddc06499841d09fdb949f54c59a14",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/telemetry-links.md",
      "line": 55,
      "row_id": "TL-6",
      "before_sha256": "9c286a831771c04e0ca3a90a77b00910751b7ddd50be57e5b4671e18fd0b9930",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/telemetry-links.md",
      "line": 56,
      "row_id": "TL-7",
      "before_sha256": "b7a6325562e2a7d73c7caaa98677dcdfc80a7329e3251e26e895772d84ec28cb",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/telemetry-links.md",
      "line": 57,
      "row_id": "TL-7-harness",
      "before_sha256": "f71fdb87e1e51b01a0a10f20ef1be21ba9c8f263a44d862d1379654bc4feb586",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/telemetry-links.md",
      "line": 58,
      "row_id": "TL-8",
      "before_sha256": "660305ede5d7cd16df97ee833435e8b2e7529cce9109ba603e4dac982a1756f2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-certifications.md",
      "line": 15,
      "row_id": "TC-M1",
      "before_sha256": "ff7037c5a44b2f3f8c99ad02baaa358d49d502dcb411bd9d99614cec2bff93b0",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-certifications.md",
      "line": 30,
      "row_id": "TC-M2",
      "before_sha256": "4e944b294b13b8e096480897535efef36ce85cea9e6c952c40b14c19d896e22c",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-certifications.md",
      "line": 38,
      "row_id": "TC-M3",
      "before_sha256": "c0aa225b80f51a2637699f04e96546c8fe16ec78a30a2f00e14ce84ec49334ce",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-certifications.md",
      "line": 46,
      "row_id": "TC-M4",
      "before_sha256": "b218e6cf329d1de644e750d10c2e0fa7eb410a7ceeb3f89451bb5c35b4b84ccf",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-intelligence.md",
      "line": 61,
      "row_id": "TQ-25",
      "before_sha256": "8e5071eb7e39b4193330974e3f245bc0cb6f91612b2d5514a5b2b6cb3f0e3e06",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-intelligence.md",
      "line": 89,
      "row_id": "TQ-M7",
      "before_sha256": "4bde48a4b9a0332c074b12f9f182e7bf42e86b393ede6c7ae27e98ac785ff878",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-intelligence.md",
      "line": 90,
      "row_id": "TQ-M8",
      "before_sha256": "533c13e2f7d6cc7bf34cb89159085c72cab63b79570b48faa82a777435138444",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-intelligence.md",
      "line": 102,
      "row_id": "TQ-M1",
      "before_sha256": "a63138ff1ce93207746f3ffa55fab19244e8a68fe7d4b12fe50f64e9d33f0ada",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-intelligence.md",
      "line": 103,
      "row_id": "TQ-M2",
      "before_sha256": "db485c744ca877fc5bc81e6665a9b8190abce80e2731363c937f38606aee5f00",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-intelligence.md",
      "line": 104,
      "row_id": "TQ-M3",
      "before_sha256": "d6d0907d451790839b98be7e74713cf3f87cce0dc742efc4fa9bbdb141f0cc16",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-intelligence.md",
      "line": 105,
      "row_id": "TQ-M4",
      "before_sha256": "22c34ace1f759dfd485075390e3998614e33dde7465fd5dad527d2d387c2862f",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-intelligence.md",
      "line": 106,
      "row_id": "TQ-M5",
      "before_sha256": "0eecb384368f71a24369859769a6b7d83d50472177f9cdcf6fa9d5de2e554689",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-intelligence.md",
      "line": 107,
      "row_id": "TQ-M6",
      "before_sha256": "28b78f78315f78798c37f163d2f3b3e3373bb076d0a2a2a7207e08c6c75d9a6d",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-run-hook.md",
      "line": 15,
      "row_id": "TR-V1",
      "before_sha256": "941f6f3da5c3bf384d843bb1efde2778fcdf1bced3ffe0b0ae7f22cd0cf073da",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-run-hook.md",
      "line": 32,
      "row_id": "TR-M1",
      "before_sha256": "83b66ae99b70aa1eedb2d6709a85c5778f065492b9d361b8f5871db2e19350f3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-run-hook.md",
      "line": 33,
      "row_id": "TR-M2",
      "before_sha256": "86aa9f57285f1cda86c9c9689314ce65b3f1a940f1d2089ed6e4e4f4cc75143e",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/test-run-hook.md",
      "line": 34,
      "row_id": "TR-M3",
      "before_sha256": "3915518f73d0ee9209d4cc133dcbcb3e30a6d5da46cc0b2dc37ec6806bf84fa9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/typed-records.md",
      "line": 20,
      "row_id": "REC-M6",
      "before_sha256": "7b2db846ae80f70e8ceae27194ecc8977cf22953a7e8a876ebedba249eb7facb",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/typed-records.md",
      "line": 29,
      "row_id": "REC-T",
      "before_sha256": "d7a503c58598ee04559dad8767362d3a615147e07f7859986a525757b5cc784b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/typed-records.md",
      "line": 34,
      "row_id": "REC-M1",
      "before_sha256": "990c9a77154cc63fc9700aeee4ef369a988d6965743f122e0078d4426c02b7e2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/typed-records.md",
      "line": 35,
      "row_id": "REC-M2",
      "before_sha256": "ae0d5f15f259c2a1e35394621e551c0f5f37d7ee4cdb325c0dfa99ae3fb385e9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/typed-records.md",
      "line": 36,
      "row_id": "REC-M3",
      "before_sha256": "ca11600eef6ae8b8456e5e2fecb4305f8134eab60a11cb4519bf59f27279d8f8",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/typed-records.md",
      "line": 37,
      "row_id": "REC-M4",
      "before_sha256": "bf2e4c46d00d28cc7312a795ad89c649c75d6c151540a2ff4c9899953bb6e3d4",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/typed-records.md",
      "line": 38,
      "row_id": "REC-M5",
      "before_sha256": "98536016d8770f5ccb62271f700d4b7ddffe00479315760e0ba4cc243304a819",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/verbs.md",
      "line": 26,
      "row_id": "VB-12",
      "before_sha256": "b8eaee6940e8b0502fbabf8e3b0c79f59d22a1146a055d8a21d5ad9c35ce5e98",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/verbs.md",
      "line": 27,
      "row_id": "VB-40",
      "before_sha256": "f41052231ffda55de0da3f175541fe02b68caeea2702799cb18b8ab569bd0e51",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/verbs.md",
      "line": 34,
      "row_id": "VB-16",
      "before_sha256": "d17b96f0d88e92a3d1c96f8176729b42e7b22b8df463426b92d3136b11062e8a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/verbs.md",
      "line": 39,
      "row_id": "VB-18",
      "before_sha256": "4be2f5d9573b5f7e6448e688a37bad9e1a9b011a3d515164b9d7aeb6c3597d61",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/verbs.md",
      "line": 46,
      "row_id": "VB-21",
      "before_sha256": "f768595846794a2b06ccab68c9c65d385c303a41f52892f3e9a6b5bcf61f19c0",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/verbs.md",
      "line": 52,
      "row_id": "VB-24",
      "before_sha256": "0cb76fcf69e530e0e4da26f99b127494cf1225fea491043cd5176aa917bd611b",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/verbs.md",
      "line": 59,
      "row_id": "VB-27",
      "before_sha256": "8caa382ac7b06363d54050f6f5086eb41a5c511186ff971a3c8e27c104d7b1e9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/verbs.md",
      "line": 64,
      "row_id": "VB-29",
      "before_sha256": "062f003e9b8cc21fba059246e54e39f372cb40e3763bc9c946965105dce07526",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/verbs.md",
      "line": 69,
      "row_id": "VB-31",
      "before_sha256": "9754db7f7c8fbe87cedffb9055d3f5a9d3b16f132674a6292704b25ae97a751a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/verbs.md",
      "line": 76,
      "row_id": "VB-35",
      "before_sha256": "31f00bac81928c0c7b5081d73217edee2cfe248b88a6c2ae5a95273dd1994a10",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/well-architected-evidence.md",
      "line": 22,
      "row_id": "WAE-M0",
      "before_sha256": "6c58fba2fe40ff66f58a9103e5bc3eb9f69ebc6fdf9e338d183d5c046cd7a5b5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/well-architected-evidence.md",
      "line": 23,
      "row_id": "WAE-M1",
      "before_sha256": "c70e870cb113af5e7f975b3420d39f3a6c5d8030b2914dd31a66d00c928e1bf3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/well-architected-evidence.md",
      "line": 38,
      "row_id": "WAE-M2",
      "before_sha256": "f17f0ec3fe313594d618e8741535917dbb8374755b15b0cff18e96ad648e2669",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/well-architected-evidence.md",
      "line": 46,
      "row_id": "WAE-M3",
      "before_sha256": "ce47477b3e307476253489c8bb75281240c2d62e9de650e4fea7fa341b49fa16",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/well-architected-evidence.md",
      "line": 47,
      "row_id": "WAE-M6",
      "before_sha256": "f04cdea530497412c5eacc9fc15e4d8bf04d59a1fdaae0874d08d54e472409d5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/well-architected-evidence.md",
      "line": 53,
      "row_id": "WAE-M4",
      "before_sha256": "9ac99194803e0ca29de7ebabdf829e2551a85d7f0e4d5e43b73c7847c54db0d3",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/well-architected-evidence.md",
      "line": 54,
      "row_id": "WAE-M5",
      "before_sha256": "ab93b6f91a5770e627ec89116ef146e353d1fca473bbef2d65566b0bedb72de5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/well-architected-evidence.md",
      "line": 55,
      "row_id": "WAE-M7",
      "before_sha256": "b4d4c28c2ac77bd8b357c7b387fdd3099735b2c96bab0494f353c0fc6bd43ca2",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/work-intelligence.md",
      "line": 62,
      "row_id": "WI-M1",
      "before_sha256": "a457b7b8ab8eba71ad347fb0eaf393ef9eecfbb03153c92abcd297094b3142d9",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/work-intelligence.md",
      "line": 63,
      "row_id": "WI-M2",
      "before_sha256": "7b830a288a85b2023bca1b00485a2068ec577277e4436cb1005f2dd08a26cfc1",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/work-intelligence.md",
      "line": 64,
      "row_id": "WI-M3",
      "before_sha256": "b4948f1ab81a12686e0d6be84e90ed7d0a15b44c61c7db83fdddedb05758c6bf",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/work-intelligence.md",
      "line": 65,
      "row_id": "WI-M4",
      "before_sha256": "334d311474146cf703f690fa0ea6216687255bbf2eeda57cff193f0eefb8cfe0",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/work-intelligence.md",
      "line": 66,
      "row_id": "WI-M5",
      "before_sha256": "b912f85e1c8d6739b31029ba0e629583baafb61bba74f672abcf2f9fd90f8e93",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/work-signals.md",
      "line": 23,
      "row_id": "WS-M1",
      "before_sha256": "a3c868fe82a60df6530056664765c57ae18131b551094653139ad491bbbecac5",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/work-signals.md",
      "line": 31,
      "row_id": "WS-M2",
      "before_sha256": "8771fd7d0775c4eb130e729dc7d5159ae6c7fc6e5ef018b13f4ea4cc5aaa1130",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/work-signals.md",
      "line": 39,
      "row_id": "WS-M3",
      "before_sha256": "421a9b954d94ae0c5239f676c7eeda20ab7906258b9520bc4e941af8e470986a",
      "gate": true,
      "lane": ""
    },
    {
      "path": "docs/plans/work-signals.md",
      "line": 44,
      "row_id": "WS-M4",
      "before_sha256": "44fda02f44abbb0811136851a013083c6da4209c588b15f7fbeca88753941563",
      "gate": true,
      "lane": ""
    }
  ]
}
~~~
