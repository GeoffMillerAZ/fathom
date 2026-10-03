# ruby_require_trace.rb — a runtime require trace as indexbench lang-oracle's neutral `imports`
# (LB-F256, design §1.3). Ruby itself evaluates the subject: every subject file is loaded in its
# own child process (`ruby -r trace_hook.rb -e 'load ARGV[0]'`), under the subject's own bundle
# (run this script with `bundle exec` in the subject root) and the load path its own test task
# gives (-I, -unit), and trace_hook.rb records each require, require_relative, load and autoload the
# file executes with the file Ruby's own lookup names for it at that moment. A child exits with
# exit!, so no test suite or at_exit hook runs, and its process group is killed after it, so no
# server a file starts at load outlives it. It runs the subject's code: use a scratch checkout.
#
#   bundle exec ruby ruby_require_trace.rb -subject <checkout> [-I <dir>]... [-r <feature>]... \
#       [-unit <dir>:<inc>,<inc>[:<feature>,<feature>]]... [-jobs N] [-timeout S] > imports.json
#
# -I dirs (subject-relative; default lib) are the load path of every file outside a -unit, and -r
# features are what its test runner loads first; a file under a -unit's dir runs from that dir with
# the unit's includes and features (RSpec's own `-I lib -I spec`, then rspec/core and its .rspec
# `--require spec_helper`, for a gem nested in the subject, as `cd <dir> && bundle exec rspec` runs
# it), under the unit's own Gemfile when it has one (`bundle install` there first). A feature loaded
# this way is the runner's, not a call site: what it requires in turn is recorded.
#
# What it writes: the executed statements with a literal path (the ones Ripper finds, the
# statements an extractor of import statements can see) whose target is a subject file. A
# statement no child executed is not written. The tier is A when the executed statements are at
# least 90% of the subject's in-repo literal statements (an unexecuted statement counts as in-repo
# when the same lookup, from this process's load path, names a subject file), else B; stderr
# names the counts and the ratio. Bench-only: never a product dependency.
require 'fileutils'
require 'json'
require 'rbconfig'
require 'ripper'

subject = nil
incs = []
reqs = []
units = []
jobs = 8
limit = 60
args = ARGV.dup
usage = "usage: bundle exec ruby #{$0} -subject <dir> [-I <dir>]... [-r <feature>]... [-unit <dir>:<inc>,...[:<feature>,...]]... [-jobs N] [-timeout S]"
until args.empty?
  case (a = args.shift)
  when '-subject' then subject = args.shift
  when '-I' then incs << args.shift
  when '-r' then reqs << args.shift
  when '-unit'
    dir, list, feats = args.shift.to_s.split(':', 3)
    abort usage if dir.to_s.empty?
    units << [dir.chomp('/'), list.to_s.split(','), feats.to_s.split(',')]
  when '-jobs' then jobs = Integer(args.shift)
  when '-timeout' then limit = Integer(args.shift)
  else abort "#{usage} (unknown #{a})"
  end
end
abort usage unless subject
ROOT = File.realpath(subject)
incs = ['lib'] if incs.empty? && File.directory?(File.join(ROOT, 'lib'))
HOOK = File.expand_path('trace_hook.rb', __dir__)

# The statements: every receiverless require, require_relative, load and autoload with a literal
# path, found by Ruby's own parser (the shapes ruby-require's dump reads).
def literal(node)
  return nil unless node.is_a?(Array) && node[0] == :string_literal
  content = node[1]
  return nil unless content.is_a?(Array) && content[0] == :string_content && content.length == 2
  part = content[1]
  part.is_a?(Array) && part[0] == :@tstring_content ? part[1] : nil
end

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

files = Dir.glob('**/*.rb', base: ROOT).reject { |p| p.split('/').any? { |s| s.start_with?('.') || %w[vendor node_modules].include?(s) } }.sort
stats = Hash.new(0)
statements = {} # [path, line, form] => literal spec
files.each do |rel|
  tree = Ripper.sexp(File.read(File.join(ROOT, rel)))
  unless tree
    stats['parse_errors'] += 1
    next
  end
  calls(tree) do |name, line, argv|
    next unless %w[require require_relative load autoload].include?(name)
    spec = literal(name == 'autoload' ? argv[1] : argv[0])
    if spec
      statements[[rel, line, name]] = spec
    else
      stats["#{name}_computed"] += 1
    end
  end
end

def unit_for(rel, units)
  units.select { |dir, _| rel.start_with?(dir + '/') }.max_by { |dir, _| dir.length }
end

# Each file in its own child: its load path, its working directory, a time limit.
out_dir = File.join(ENV.fetch('TMPDIR', '/tmp'), "ruby-require-trace-#{Process.pid}")
Dir.mkdir(out_dir) unless Dir.exist?(out_dir)
queue = Queue.new
files.each_with_index { |f, i| queue << [f, i] }
results = {}
lock = Mutex.new
workers = Array.new([jobs, 1].max) do
  Thread.new do
    loop do
      rel, i = begin
        queue.pop(true)
      rescue ThreadError
        break
      end
      unit = unit_for(rel, units)
      cwd = unit ? File.join(ROOT, unit[0]) : ROOT
      dirs = (unit ? unit[1] : incs).map { |d| File.expand_path(d, cwd) }
      feats = unit ? unit[2] : reqs
      trace = File.join(out_dir, "#{i}.jsonl")
      env = { 'RUBY_REQUIRE_TRACE_ROOT' => ROOT, 'RUBY_REQUIRE_TRACE_OUT' => trace, 'RUBY_REQUIRE_TRACE_FEATURES' => feats.join(',') }
      # A unit with a Gemfile of its own runs under its own bundle (`cd <dir> && bundle exec rspec`).
      env['BUNDLE_GEMFILE'] = File.join(cwd, 'Gemfile') if unit && File.file?(File.join(cwd, 'Gemfile'))
      cmd = [RbConfig.ruby, *dirs.flat_map { |d| ['-I', d] }, '-r', HOOK, '-e',
             'begin; ENV["RUBY_REQUIRE_TRACE_FEATURES"].split(",").each { |f| require f }; load File.expand_path(ARGV[0]); ' \
             'rescue Exception => e; $stderr.puts "#{e.class}: #{e.message[0, 200]}"; exit!(3); end; exit!(0)',
             File.join(ROOT, rel)]
      errf = File.join(out_dir, "#{i}.err")
      # Its own process group: whatever the file started (a test that boots a server at load) is
      # killed with it.
      pid = Process.spawn(env, *cmd, chdir: cwd, in: File::NULL, out: File::NULL, err: errf, pgroup: true)
      waiter = Process.detach(pid)
      status = if waiter.join(limit)
                 waiter.value.exitstatus
               else
                 :timeout
               end
      Process.kill('KILL', -pid) rescue nil
      waiter.join
      err = File.exist?(errf) ? File.read(errf) : ''
      lock.synchronize { results[rel] = [status, err.lines.last.to_s.strip, trace] }
    end
  end
end
workers.each(&:join)

executed = {} # [path, line, form] => [to_path, outside]
results.sort.each do |rel, (status, err, trace)|
  case status
  when 0 then stats['files_loaded'] += 1
  when :timeout then stats['files_timed_out'] += 1
  else
    stats['files_raised'] += 1
    warn "raised #{rel}: #{err}" if ENV['RUBY_REQUIRE_TRACE_VERBOSE']
  end
  next unless File.exist?(trace)
  File.foreach(trace) do |l|
    r = JSON.parse(l)
    key = [r['path'], r['line'], r['form']]
    prev = executed[key]
    executed[key] = [r['to_path'], r['outside']] if prev.nil? || (prev[0].nil? && r['to_path'])
  end
end

# The same lookup from this process (bundler's load path plus the -I dirs) for a statement no
# child executed: in-repo or not, for the attestation ratio only.
$LOAD_PATH.unshift(*incs.map { |d| File.expand_path(d, ROOT) })
def resolve_static(form, rel, spec, units, incs)
  path = case form
         when 'require_relative' then File.expand_path(spec, File.dirname(File.join(ROOT, rel)))
         when 'load'
           cands = spec.start_with?('/') ? [spec] : [File.join(ROOT, spec)] + $LOAD_PATH.map { |d| File.join(d.to_s, spec) }
           return cands.find { |c| File.file?(c) }
         else spec
         end
  saved = $LOADED_FEATURES.dup
  $LOADED_FEATURES.clear
  if (unit = unit_for(rel, units))
    extra = unit[1].map { |d| File.expand_path(d, File.join(ROOT, unit[0])) }
    $LOAD_PATH.unshift(*extra)
  end
  r = $LOAD_PATH.resolve_feature_path(path)
  r && r[0] == :rb ? r[1] : nil
rescue LoadError
  nil
ensure
  $LOAD_PATH.shift(extra.length) if extra
  $LOADED_FEATURES.replace(saved) if saved
end

def in_repo(target)
  return nil unless target && File.exist?(target)
  real = File.realpath(target)
  real.start_with?(ROOT + '/') ? real[(ROOT.length + 1)..] : nil
end

imports = []
in_repo_total = 0
attested = 0
statements.each do |(rel, line, form), spec|
  if (hit = executed[[rel, line, form]])
    to, outside = hit
    if to
      in_repo_total += 1
      attested += 1
      stats["#{form}_in_subject"] += 1
      imports << { 'path' => rel, 'line' => line, 'to_path' => to, 'module' => spec }
    else
      stats[outside ? "#{form}_outside" : "#{form}_unresolved"] += 1
    end
  elsif (to = in_repo(resolve_static(form, rel, spec, units, incs)))
    in_repo_total += 1
    stats["#{form}_unexecuted_in_subject"] += 1
    warn "unexecuted #{rel}:#{line} #{form} #{spec} -> #{to}" if ENV['RUBY_REQUIRE_TRACE_VERBOSE']
  else
    stats["#{form}_unexecuted_elsewhere"] += 1
  end
end
executed.each_key { |k| stats['computed_executed'] += 1 unless statements.key?(k) }
imports.sort_by! { |i| [i['path'], i['line'], i['to_path'], i['module']] }
ratio = in_repo_total.zero? ? 0.0 : attested.to_f / in_repo_total
tier = ratio >= 0.90 ? 'A' : 'B'
warn "files #{files.length} " + stats.sort.map { |k, v| "#{k} #{v}" }.join(' ') +
     format(' attested %d of %d in-repo statements (%.4f): tier %s', attested, in_repo_total, ratio, tier)
bundler = defined?(Bundler::VERSION) ? ", bundler #{Bundler::VERSION}" : ''
out = { 'tool' => 'ruby-require-trace', 'version' => "ruby #{RUBY_VERSION}#{bundler}", 'tier' => tier, 'imports' => imports }
puts JSON.pretty_generate(out)
FileUtils.rm_rf(out_dir)
