class Outer::Reopened < Base
  def method_missing(name, *args, &blk)
    super
  end
end

module Deep::Nested::Mod
  def self.helper
    Deep::Nested.sibling
  end
end

def obj.custom(x); x; end

class Tiny; end
module Util; def self.a; end; def self.b; end; end
