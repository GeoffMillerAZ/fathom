# Native events only; syntax/Fathom outputs are never read by this hook.
# Load no gems before Bundler establishes the subject's declared versions.
abort 'Ruby 3.3.8 required' unless RUBY_VERSION == '3.3.8'
expand_path = File.method(:expand_path)
root = File.realpath(ENV.fetch('RUBY_ORACLE_SUBJECT')) + '/'
out = ENV.fetch('RUBY_ORACLE_TRACE_DIR')
rows = {}
entries = 0
refused = false
owner_pid = Process.pid
row_cap = Integer(ENV.fetch('RUBY_ORACLE_ROW_CAP', '1000000'))
function_cap = Integer(ENV.fetch('RUBY_ORACLE_FUNCTION_CAP', '100000'))
abort 'invalid native caps' unless (1..1_000_000).cover?(row_cap) && (1..100_000).cover?(function_cap)
trace = TracePoint.new(:call, :c_call) do |event|
  if owner_pid != Process.pid
    owner_pid = Process.pid
    rows = {}
    entries = 0
    refused = false
  end
  frames = caller_locations(0, 3)
  caller_frame = frames[event.event == :call ? 2 : 1]
  next unless caller_frame
  path = expand_path.call(caller_frame.absolute_path || caller_frame.path)
  next unless path.start_with?(root)
  caller_name = caller_frame.base_label
  next if caller_name.start_with?('<')
  entries += 1
  rows[[path.delete_prefix(root), caller_frame.lineno, caller_name, event.method_id.to_s, event.callee_id.to_s]] = true
  if rows.size > row_cap
    refused = true
    trace.disable
  end
end
trace.enable
at_exit do
  trace.disable
  functions = {}
  ObjectSpace.each_object(Module) do |mod|
    begin
      names = (mod.instance_methods(false) + mod.private_instance_methods(false) + mod.protected_instance_methods(false)).uniq
      methods = names.map { |name| mod.instance_method(name) }
      methods += mod.singleton_methods(false).map { |name| mod.method(name) }
      methods.each do |method|
        location = method.source_location
        next unless location && expand_path.call(location[0]).start_with?(root)
        iseq = RubyVM::InstructionSequence.of(method)
        next unless iseq
        code_location = iseq.to_a[4][:code_location]
        next unless code_location
        functions[[expand_path.call(location[0]).delete_prefix(root), method.original_name.to_s,
                   code_location[0], code_location[2]]] = true
        if functions.size > function_cap
          refused = true
          break
        end
      end
    rescue NameError, TypeError
      # A module can remove/replace methods while being enumerated. Those
      # native ranges remain unattested; never invent a parser fallback.
    end
    break if refused
  end
  receipt = {ruby:RUBY_VERSION,pid:Process.pid,native_entries:entries,refused:refused,
    calls:refused ? [] : rows.keys.sort,functions:refused ? [] : functions.keys.sort}
  require 'json'
  File.write(File.join(out, "native-#{Process.pid}.json"), JSON.generate(receipt))
end
