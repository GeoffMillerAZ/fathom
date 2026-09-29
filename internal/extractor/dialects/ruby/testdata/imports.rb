require 'json'
require "net/http"
require_relative 'support/util'
require_relative "../lib/other.rb"
load 'tasks/setup.rb'
autoload :Lazy, 'app/lazy'
require File.expand_path('config', __dir__)
require "dynamic/#{name}"
YAML.load('a: 1')
module App
  def self.boot
    require 'set'
  end
end
