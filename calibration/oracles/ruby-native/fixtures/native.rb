class NativeParent
  VALUE = 7
  def inherited
    'x'.length
  end
end
class NativeChild < NativeParent
  alias alternate inherited
  def driver
    inherited
    alternate
    NativeParent::VALUE
    unknown_receiver.unresolvable_call
  end
end
NativeChild.new.driver if false
