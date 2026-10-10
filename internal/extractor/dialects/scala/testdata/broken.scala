package broken

object Before {
  def ok(): Int = 1
}

class Broken {
  def bad(: Int = = 2
}

object After {
  def fine(x: Int): Int = x
}
