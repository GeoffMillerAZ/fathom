package calls

object Calls {
  def helper(x: Int): Int = x

  def plain(): Int = helper(1)

  def member(xs: List[Int]): List[Int] =
    xs.map(helper).filter(_ > 0)

  def generic(): Unit = {
    val a = List.empty[Int]
    identity[Int](3)
    a.headOption.fold(0)(helper)
    Some(2)
    new StringBuilder("x")
    xs foldLeft 0
    println(s"$a")
  }

  def curried(a: Int)(b: Int): Int = a + b
  def applyCurried(): Int = curried(1)(2)

  def nested(): Int = {
    def inner(y: Int): Int = helper(y)
    inner(4)
  }

  def block(): Unit = run { () =>
    helper(5)
  }

  val notCallable: Int = helper(6)

  def withDefault(n: Int = helper(7)): Int = n

  def viaSuper(): String = super.toString()
}

class Overrides(x: Int) extends Base(x) {
  def this() = this(0)
  helper(8)
  override def go(): Unit = {
    this.go()
    Calls.plain()
  }
}
