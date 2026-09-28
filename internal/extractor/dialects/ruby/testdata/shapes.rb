require 'json'
require_relative 'lib/helper'

module Outer
  VERSION = '1.0'

  module Inner
    class Deep
      def call(arg)
        helper(arg)
        JSON.parse(arg)
      end
    end
  end

  class Service < Base
    include Enumerable
    attr_accessor :name

    def initialize(name)
      @name = name
      super()
    end

    def self.build(*args)
      new(*args).tap { |s| s.configure }
    end

    def each(&block)
      items.each(&block)
    end

    class << self
      def singleton_helper
        compute
      end
    end

    private

    def secret?
      true
    end

    def value=(v)
      @value = v
    end
  end
end

class Outer::Reopened
  def method_missing(name, *args)
    super
  end
end

def top_level
  puts 'hi'
end
