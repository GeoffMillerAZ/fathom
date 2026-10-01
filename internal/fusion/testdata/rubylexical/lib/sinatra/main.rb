require 'sinatra/base'

module Sinatra
  class Application < Base
    def handle
      Request.new
    end
  end
end
