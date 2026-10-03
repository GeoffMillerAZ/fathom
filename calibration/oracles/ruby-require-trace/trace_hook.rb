# trace_hook.rb — loaded with `ruby -r` into each child ruby_require_trace.rb runs: every
# require, require_relative, load and autoload a subject file executes is recorded with the file
# Ruby's own lookup names for it at that moment, under the program's own load path ($LOAD_PATH as
# bundler and the subject's code have set it). One JSON line per call site is appended to
# $RUBY_REQUIRE_TRACE_OUT as the call runs, so a file that raises later keeps what it executed.
# Bench-only: never a product dependency.
require 'json'

module RubyRequireTrace
  ROOT = File.realpath(ENV.fetch('RUBY_REQUIRE_TRACE_ROOT')) + '/'
  OUT = File.open(ENV.fetch('RUBY_REQUIRE_TRACE_OUT'), 'a')
  OUT.sync = true

  module_function

  # site is the subject-relative path and line of the call, nil when the caller is not a
  # subject file (a gem, the standard library, this hook's own require_relative).
  def site(locs)
    loc = locs.first
    return nil unless loc&.absolute_path && loc.absolute_path != __FILE__
    path = begin
      File.realpath(loc.absolute_path)
    rescue SystemCallError
      return nil
    end
    return nil unless path.start_with?(ROOT)
    [path[ROOT.length..], loc.lineno]
  end

  # resolve_feature is `require`'s own lookup over the load path as it stands, with
  # $LOADED_FEATURES emptied for the lookup (a feature already loaded answers false).
  def resolve_feature(feature)
    saved = $LOADED_FEATURES.dup
    $LOADED_FEATURES.clear
    r = $LOAD_PATH.resolve_feature_path(feature)
    r && r[0] == :rb && r[1].is_a?(String) ? r[1] : nil
  rescue LoadError, TypeError
    nil
  ensure
    $LOADED_FEATURES.replace(saved) if saved
  end

  # resolve_load is `load`'s lookup: an absolute or ./ ../ path from the working directory,
  # else each load-path directory, then the working directory; no extension is added.
  def resolve_load(file)
    file = file.to_path if file.respond_to?(:to_path)
    return nil unless file.is_a?(String)
    cands = if file.start_with?('/') then [file]
            elsif file.start_with?('./', '../') then [File.expand_path(file)]
            else $LOAD_PATH.map { |d| File.join(d.to_s, file) } + [File.expand_path(file)]
            end
    cands.find { |c| File.file?(c) }
  end

  def record(form, at, spec, target)
    real = target && File.exist?(target) ? File.realpath(target) : nil
    to = real && real.start_with?(ROOT) ? real[ROOT.length..] : nil
    OUT.puts(JSON.generate({ 'form' => form, 'path' => at[0], 'line' => at[1], 'module' => spec.to_s,
                             'to_path' => to, 'outside' => !real.nil? && to.nil? }))
  rescue StandardError
    nil
  end
end

module RubyRequireTraceKernel
  def require(feature)
    if (at = RubyRequireTrace.site(caller_locations(1, 1)))
      target = RubyRequireTrace.resolve_feature(feature)
      begin
        return super
      ensure
        target ||= RubyRequireTrace.resolve_feature(feature) # a gem the call itself activated
        RubyRequireTrace.record('require', at, feature, target)
      end
    end
    super
  end

  def require_relative(feature)
    loc = caller_locations(1, 1).first
    base = loc&.absolute_path
    raise LoadError, 'cannot infer basepath' unless base
    abs = File.expand_path(feature.to_s, File.dirname(File.realpath(base)))
    if (at = RubyRequireTrace.site([loc]))
      RubyRequireTrace.record('require_relative', at, feature, RubyRequireTrace.resolve_feature(abs))
    end
    require(abs)
  end

  def load(file, *rest)
    if (at = RubyRequireTrace.site(caller_locations(1, 1)))
      RubyRequireTrace.record('load', at, file, RubyRequireTrace.resolve_load(file))
    end
    super
  end

  def autoload(const, path)
    if (at = RubyRequireTrace.site(caller_locations(1, 1)))
      RubyRequireTrace.record('autoload', at, path, RubyRequireTrace.resolve_feature(path))
    end
    super
  end
end

module RubyRequireTraceModule
  def autoload(const, path)
    if (at = RubyRequireTrace.site(caller_locations(1, 1)))
      RubyRequireTrace.record('autoload', at, path, RubyRequireTrace.resolve_feature(path))
    end
    super
  end
end

Kernel.prepend(RubyRequireTraceKernel)
Kernel.singleton_class.prepend(RubyRequireTraceKernel)
Module.prepend(RubyRequireTraceModule)
