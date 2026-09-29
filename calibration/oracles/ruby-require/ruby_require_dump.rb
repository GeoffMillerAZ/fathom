# ruby_require_dump.rb — Ruby's own require resolution as indexbench lang-oracle's
# neutral `imports` (LB-F40, design §1.2 import_resolution). Tier B: Ruby's parser
# (Ripper) finds each require, require_relative, load and autoload with a literal path,
# and Ruby's own feature lookup resolves it — over the load path this run is given,
# which is the operator's configuration, not the program's (see README.md).
#
#   ruby ruby_require_dump.rb -subject <checkout> [-I <subject-relative dir>]... > imports.json
#
# Only imports resolving to a file inside the subject are written; the rest are counted
# on stderr. Bench-only: never a product dependency.
require 'ripper'
require 'json'

subject = nil
incs = []
args = ARGV.dup
until args.empty?
  case (a = args.shift)
  when '-subject' then subject = args.shift
  when '-I' then incs << args.shift
  else abort "usage: ruby #{$0} -subject <dir> [-I <dir>]... (unknown #{a})"
  end
end
abort "usage: ruby #{$0} -subject <dir> [-I <dir>]..." unless subject
root = File.realpath(subject)
incs = ['lib'] if incs.empty? && File.directory?(File.join(root, 'lib'))
load_dirs = incs.map { |d| File.expand_path(d, root) }
$LOAD_PATH.unshift(*load_dirs)

# resolve_feature is Ruby's own `require` lookup: RubyVM.resolve_feature_path (2.6),
# $LOAD_PATH.resolve_feature_path (2.7+). nil when Ruby finds no such feature. The
# lookup answers `false` for a feature this process already loaded (json, ripper), so
# it runs with $LOADED_FEATURES emptied: the subject's program has loaded nothing.
def resolve_feature(feature)
  saved = $LOADED_FEATURES.dup
  $LOADED_FEATURES.clear
  r = if $LOAD_PATH.respond_to?(:resolve_feature_path)
        $LOAD_PATH.resolve_feature_path(feature)
      else
        RubyVM.resolve_feature_path(feature)
      end
  r && r[0] == :rb && r[1].is_a?(String) ? r[1] : nil
rescue LoadError
  nil
ensure
  $LOADED_FEATURES.replace(saved)
end

# resolve_load is `load`'s lookup: an absolute or ./ ../ path as is, else each load-path
# directory and then the working directory (the subject root); no extension is added.
def resolve_load(path, root, load_dirs)
  cands = if path.start_with?('/') then [path]
          elsif path.start_with?('./', '../') then [File.expand_path(path, root)]
          else load_dirs.map { |d| File.join(d, path) } + [File.join(root, path)]
          end
  cands.find { |c| File.file?(c) }
end

# literal is a plain string argument's text, nil for anything computed.
def literal(node)
  return nil unless node.is_a?(Array) && node[0] == :string_literal
  content = node[1]
  return nil unless content.is_a?(Array) && content[0] == :string_content && content.length == 2
  part = content[1]
  part.is_a?(Array) && part[0] == :@tstring_content ? part[1] : nil
end

# calls yields [method name, line, argument nodes] for every receiverless call.
def calls(node, &blk)
  return unless node.is_a?(Array)
  case node[0]
  when :command
    id = node[1]
    if id.is_a?(Array) && id[0] == :@ident && node[2].is_a?(Array) && node[2][0] == :args_add_block
      yield id[1], id[2][0], node[2][1]
    end
  when :method_add_arg
    fc = node[1]
    if fc.is_a?(Array) && fc[0] == :fcall && fc[1].is_a?(Array) && fc[1][0] == :@ident
      paren = node[2]
      block = paren.is_a?(Array) && paren[0] == :arg_paren ? paren[1] : nil
      yield fc[1][1], fc[1][2][0], block[1] if block.is_a?(Array) && block[0] == :args_add_block
    end
  end
  node.each { |c| calls(c, &blk) }
end

files = Dir.glob('**/*.rb', base: root).reject { |p| p.split('/').any? { |s| s.start_with?('.') || %w[vendor node_modules].include?(s) } }.sort
imports = []
stats = Hash.new(0)
files.each do |rel|
  src = File.read(File.join(root, rel))
  tree = Ripper.sexp(src)
  unless tree
    stats['parse_errors'] += 1
    next
  end
  calls(tree) do |name, line, argv|
    next unless %w[require require_relative load autoload].include?(name)
    arg = name == 'autoload' ? argv[1] : argv[0]
    spec = literal(arg)
    unless spec
      stats["#{name}_computed"] += 1
      next
    end
    target = case name
             when 'require_relative' then resolve_feature(File.expand_path(spec, File.dirname(File.join(root, rel))))
             when 'load' then resolve_load(spec, root, load_dirs)
             else resolve_feature(spec)
             end
    real = target && File.exist?(target) ? File.realpath(target) : nil
    if real.nil?
      stats["#{name}_unresolved"] += 1
    elsif !real.start_with?(root + '/')
      stats["#{name}_outside"] += 1
    else
      stats["#{name}_in_subject"] += 1
      imports << { 'path' => rel, 'line' => line, 'to_path' => real[(root.length + 1)..-1], 'module' => spec }
    end
  end
end
imports.sort_by! { |i| [i['path'], i['line'], i['to_path'], i['module']] }
warn "files #{files.length} " + stats.sort.map { |k, v| "#{k} #{v}" }.join(' ')
out = { 'tool' => 'ruby-require', 'version' => "ruby #{RUBY_VERSION}", 'tier' => 'B', 'imports' => imports }
puts JSON.pretty_generate(out)
