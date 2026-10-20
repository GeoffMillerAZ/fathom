Epoch 68 (wave lang4, one bump for the wave, made by the integrator). Item star (LB-Q-bazel,
LB-Q-tiltfile): BUILD, WORKSPACE, MODULE.bazel, .bzl and Tiltfile files read through the starlark
tree-sitter dialects. Item jss (LB-Q-json-schema): a .json whose root $schema is a json-schema.org
URL, or whose name ends .schema.json, reads through the json-schema dialect. Critic (O379): the
file-resolve pass keys a file a dialect claims by basename (BUILD, meson.build, CMakeLists.txt) as
well as by extension. Every such file fingerprinted before holds the old extraction.
