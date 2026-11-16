// conceptInventoryConflict: #ConceptInventory & {
//     concepts: {
//         "K": {
//             required_by: [{design_row: "BE-D2"}]
//             expectations: [{metric: "cross_file_paths", unit: "paths", minimum: 30}]
//         }
//         "K": {expectations: [{metric: "cross_file_paths", unit: "paths", minimum: 31}]}
//     }
//     groups: []
// }

// cases: [
// {"name":"required member","path":"/environments/0/id","value":null,"remove":true},
// {"name":"array type","path":"/environments","value":null,"remove":false},
// {"name":"nonempty array","path":"/environments/0/repositories","value":[],"remove":false},
// {"name":"enum","path":"/environments/0/tier","value":"unknown","remove":false},
// {"name":"text","path":"/requirements/0/checker/version","value":"","remove":false},
// {"name":"commit","path":"/environments/0/repositories/0/commit","value":"bad","remove":false},
// {"name":"digest","path":"/requirements/0/checker/sha256","value":"bad","remove":false},
// {"name":"minimum","path":"/requirements/0/positive_minimum","value":29,"remove":false},
// {"name":"count","path":"/environments/0/scale/lines","value":-1,"remove":false},
// {"name":"start line","path":"/witnesses/0/anchor/start_line","value":0,"remove":false},
// {"name":"end line","path":"/witnesses/0/anchor/end_line","value":0,"remove":false},
// {"name":"authored pass","path":"/pass","value":false,"remove":false},
// {"name":"missing environment","path":"/witnesses/0/environment/id","value":"missing-e","remove":false},
// {"name":"missing requirement","path":"/witnesses/0/requirement/id","value":"missing-q","remove":false},
// {"name":"anchor pin","path":"/witnesses/0/anchor/repository/commit","value":"cccccccccccccccccccccccccccccccccccccccc","remove":false},
// {"name":"checker pin","path":"/witnesses/0/checker/version","value":"2","remove":false},
// {"name":"stratum language","path":"/coverage/0/stratum/language","value":"python","remove":false},
// {"name":"stratum prediction","path":"/coverage/0/stratum/prediction","value":"null","remove":false},
// {"name":"stratum subtype","path":"/coverage/0/stratum/subtype","value":"long","remove":false},
// {"name":"card identity","path":"/requirements/0/card/id","value":"other","remove":false},
// {"name":"stratum scale","path":"/coverage/0/stratum/scale/0/class","value":"M","remove":false},
// {"name":"capability","path":"/coverage/0/capability","value":"other","remove":false},
// {"name":"canonical path","path":"/witnesses/0/anchor/path","value":"../x","remove":false},
// {"name":"public URL","path":"/environments/0/repositories/0/url","value":"","remove":false},
// ]
