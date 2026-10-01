# precommit_psych.rb — the tier-C half of the git-hooks-config oracle: a Ruby Psych
# reference parse (Psych ships with Ruby; libyaml is YAML's reference C parser) of a
# pre-commit config or hook manifest, independent of fathom's decode (gopkg.in/yaml.v3).
#
#   ruby precommit_psych.rb <file> config|manifest
#
# Prints one JSON array of {"name", "line", "kind"}: for a config
# (.pre-commit-config.yaml), every mapping of `repos[*].hooks[*]` that carries an `id`;
# for a manifest (.pre-commit-hooks.yaml), every mapping of the root sequence that
# carries one. The line is the mapping's first line (Psych's start_line, 0-based, + 1).
# Aliases are followed; a file Psych cannot parse exits 1 with the error on stderr.
require "psych"
require "json"

def resolve(node, anchors)
  node.is_a?(Psych::Nodes::Alias) ? anchors[node.anchor] : node
end

def collect_anchors(node, anchors)
  return if node.nil?
  anchors[node.anchor] = node if node.respond_to?(:anchor) && node.anchor && !node.is_a?(Psych::Nodes::Alias)
  (node.children || []).each { |c| collect_anchors(c, anchors) }
end

def get(map, key, anchors)
  return nil unless map.is_a?(Psych::Nodes::Mapping)
  map.children.each_slice(2) do |k, v|
    return resolve(v, anchors) if k.respond_to?(:value) && k.value == key
  end
  nil
end

def hook_def(map, anchors)
  return nil unless map.is_a?(Psych::Nodes::Mapping)
  id = get(map, "id", anchors)
  return nil unless id.is_a?(Psych::Nodes::Scalar) && id.value != ""
  { "name" => id.value, "line" => map.start_line + 1, "kind" => "job" }
end

path, mode = ARGV
begin
  doc = Psych.parse(File.read(path))
rescue Psych::SyntaxError => e
  warn e.message
  exit 1
end
out = []
if doc && doc.root
  anchors = {}
  collect_anchors(doc.root, anchors)
  root = resolve(doc.root, anchors)
  case mode
  when "config"
    repos = get(root, "repos", anchors)
    if repos.is_a?(Psych::Nodes::Sequence)
      repos.children.each do |repo|
        hooks = get(resolve(repo, anchors), "hooks", anchors)
        next unless hooks.is_a?(Psych::Nodes::Sequence)
        hooks.children.each do |h|
          d = hook_def(resolve(h, anchors), anchors)
          out << d if d
        end
      end
    end
  when "manifest"
    if root.is_a?(Psych::Nodes::Sequence)
      root.children.each do |h|
        d = hook_def(resolve(h, anchors), anchors)
        out << d if d
      end
    end
  else
    warn "mode must be config or manifest"
    exit 2
  end
end
puts JSON.generate(out)
