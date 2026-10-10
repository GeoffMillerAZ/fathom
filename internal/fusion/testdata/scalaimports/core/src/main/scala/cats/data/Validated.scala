package cats.data

sealed abstract class Validated[+E, +A]

object Validated {
  def valid[A](a: A): Validated[Nothing, A] = Valid(a)
}

final case class Valid[+A](a: A) extends Validated[Nothing, A]
