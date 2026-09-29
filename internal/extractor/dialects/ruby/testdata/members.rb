# Definitions beyond def/class/module: constants, accessors, aliases, dynamic classes.
module Config
  VERSION = '2.1'
  LIMITS = { soft: 1, hard: 2 }.freeze
  LOW, HIGH = 1, 9
  Retries ||= 3
  Config::MARKER = :marker
  Point = Struct.new(:x, :y)
  Pair = Data.define(:left, :right)
  Failure = Class.new(StandardError)
  Mixin = Module.new do
    def shared; end
  end

  class Record < Base
    attr_accessor :name, :email
    attr_reader :id
    attr_writer :note
    attr :legacy

    def <=>(other)
      id <=> other.id
    end

    def name=(value)
      @name = value
    end

    def valid?; true; end

    def sq(x) = x * x

    alias label name
    alias_method :title, :name
    alias_method :legacy_id, :id
    define_method(:dyn) { |a| helper(a) }
    define_method('quoted') { 1 }

    class << self
      attr_accessor :registry

      def build(**opts)
        new(**opts)
      end
    end

    private def secret
      :s
    end
  end
end

anon = Class.new(Config::Record) do
  def hook; end
end
plug = Module.new
