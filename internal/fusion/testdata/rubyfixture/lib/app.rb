require 'json'
require_relative 'app/helper'
require 'app/models/user'

class App
  def run(raw)
    user = User.new(JSON.parse(raw))
    helper(user)
    Formatting.money(1)
    skipped(user)
  end
end
