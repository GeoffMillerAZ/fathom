require 'set'

class Pipeline
  STAGES = %i[parse check emit].freeze

  def run(input, *rest, verbose: false,
          &block)
    tokens = Lexer.scan(input)
    tree = Parser::Core.build(tokens)
    puts tree if verbose
    tree&.walk { |n| visit(n) }
    STAGES.each { |s| dispatch(s) }
    Registry.instance.register(self)
    ::Kernel.exit(0) unless block
    super
  end

  def dispatch(stage)
    handler = handlers.fetch(stage)
    handler.call(stage)
    raise ArgumentError, 'no handler' if handler.nil?
  end

  private

  def handlers
    @handlers ||= Hash.new
  end
end
