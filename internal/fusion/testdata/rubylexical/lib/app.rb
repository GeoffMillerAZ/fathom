require 'sinatra/helpers'

class App
  def stream
    Sinatra::Helpers::Stream.new
  end
end
