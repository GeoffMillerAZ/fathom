top = 1

class Widget
  attr_reader :name

  prefix = "w"
  define_method(:tag) { prefix + suffix }

  def initialize(name, size = 1, *rest, key:, opt: 2, **extra, &blk)
    @name = name
    total = size + opt
    helper
    finish(total, rest, key, extra, blk) if ready?
    def blk.each; yield(go) end
  end

  def render
    label = name
    label.upcase
    format_label
    [1, 2].each { |item| show(item, label, counter) }
    counter = 0
    counter += 1
    begin
      risky
    rescue StandardError => err
      err.message
    end
    for row in rows
      row.draw
    end
    case shape
    in [first, *others] then first.fit(others)
    in {width:} then width
    in {height: Integer => tall} then tall
    end
    scaled = ->(x) { x * factor }
    scaled.call(counter)
  end

  def name=(value)
    @name = value
  end

  def other
    label
  end

  def obj.skipped
    hidden
  end
end

def uses_top
  top
end
