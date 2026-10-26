# Released owner analyzer answers only; never reads Fathom output.
require 'json'
require 'pathname'
require 'set'
gem 'prism', '=2.0.0'
gem 'rbs', '=4.2.0'
gem 'typeprof', '=0.33.2'
require 'typeprof'
abort 'wrong TypeProf release' unless TypeProf::VERSION == '0.33.2'
abort 'wrong Ruby release' unless RUBY_VERSION == '3.3.8'
fixture = ARGV.delete('--fixture')
root = File.realpath(ARGV.fetch(0))
if fixture
  relative_paths = ['native.rb']
else
  top = IO.popen(['git', '-C', root, 'rev-parse', '--show-toplevel'], &:read).strip
  abort 'subject must be its Git checkout root' unless File.realpath(top) == root
  relative_paths = IO.popen(['git', '-C', root, 'ls-files', '-z'], &:read).split("\0")
end
paths = relative_paths.filter_map do |rel|
  path = File.join(root, rel)
  next unless File.file?(path) && !File.symlink?(path)
  next if rel.split('/').any? { |s| %w[.git vendor node_modules].include?(s) }
  head = File.open(path, 'rb') { |f| f.read(256) } || ''
  eligible = %w[.rb .rake .ru].include?(File.extname(rel)) || File.basename(rel) == 'Rakefile' || head.match?(/\A#![^\n]*\b(?:j?ruby)\b/)
  path if eligible
end.sort
abort 'whole-unit refusal: empty source population' if paths.empty?
file_cap = Integer(ENV.fetch('TYPEPROF_FILE_CAP', '2048'))
byte_cap = Integer(ENV.fetch('TYPEPROF_BYTE_CAP', (64*1024*1024).to_s))
pair_cap = Integer(ENV.fetch('TYPEPROF_PAIR_CAP', '1000000'))
abort 'invalid TypeProf caps' unless (1..2048).cover?(file_cap) && (1..64*1024*1024).cover?(byte_cap) && (1..1_000_000).cover?(pair_cap)
abort 'whole-unit refusal: file count cap' if paths.size > file_cap
abort 'whole-unit refusal: source byte cap' if paths.sum { |p| File.size(p) } > byte_cap
service = TypeProf::Core::Service.new(position_encoding: Encoding::UTF_8, rbs_collection: nil)
loaded, rejected = [], []
paths.each do |path|
  STDERR.puts "analyze #{loaded.size+rejected.size+1}/#{paths.size}"
  if service.update_file(path, nil)
    loaded << path
  else
    rejected << path
  end
end
asts = service.instance_variable_get(:@rb_text_nodes)
methods = []
asts.each do |path, ast|
  ast.traverse do |event, node|
    next unless event == :enter
    node.boxes(:mdef) { |mdef| methods << [path, mdef.node.code_range, mdef.mid.to_s] }
  end
end
methods.uniq!
pairs, sites, unresolved = Set.new, [], []
ambiguous_callers = 0
asts.each do |path, ast|
  ast.traverse do |event, node|
    next unless event == :enter
    next unless node.is_a?(TypeProf::Core::AST::CallNode) || node.is_a?(TypeProf::Core::AST::ConstantReadNode)
    range = node.code_range
    scopes = methods.select { |p, r, _| p == path && r.first <= range.first && range.last <= r.last }
    next if scopes.empty?
    rel = Pathname.new(path).relative_path_from(Pathname.new(root)).to_s
    loc = [rel, range.first.lineno, range.first.column, range.last.lineno, range.last.column]
    sites << loc
    width = scopes.map { |_, r, _| [r.last.lineno-r.first.lineno, r.last.column-r.first.column] }.min
    names = scopes.select { |_, r, _| [r.last.lineno-r.first.lineno, r.last.column-r.first.column] == width }.map(&:last).uniq
    unless names.size == 1
      ambiguous_callers += 1
      unresolved << loc
      next
    end
    caller_name = names.first
    answers = Set.new
    if node.is_a?(TypeProf::Core::AST::CallNode)
      node.boxes(:mcall) do |box|
        # Mirror Service#definitions' own Class#new handling: use the native
        # initialize box when present, not an extra syntactic `new` target.
        boxes = box.changes.boxes.filter_map do |key, inner|
          inner if key[0] == :mcall && key[3] == :initialize
        end
        boxes << box if boxes.empty?
        boxes.each do |resolved_box|
          resolved_box.resolve(service.genv, nil) do |me, _ty, mid, _orig_ty|
            next unless me && (!me.decls.empty? || !me.defs.empty?)
            answers << mid.to_s
          end
        end
      end
    else
      node.boxes(:cread) do |box|
        cdef = box.const_read&.cdef
        answers << box.const_read.name.to_s if cdef && cdef.exist?
      end
    end
    unresolved << loc if answers.empty?
    answers.each { |target| pairs << [rel, caller_name, target] }
    abort 'whole-unit refusal: native answer count cap' if pairs.size > pair_cap
  end
end
puts JSON.generate(tool: 'ruby/typeprof native MethodCallBox resolution', version: TypeProf::VERSION,
 tier: 'A', calls: pairs.to_a.sort.map { |p,f,t| {path:p,from:f,to:t} },
 receipt: {files:paths.size,bytes:paths.sum{|p|File.size(p)},loaded:loaded.size,rejected:rejected.size,
 native_scope_sites:sites.size,unresolved_native_sites:unresolved.size,ambiguous_native_callers:ambiguous_callers,
 unresolved_policy:'misses, never dropped',
 original_full_population_audit:'measure_typeprof.py records every unmatched independent written pair as a miss'})
