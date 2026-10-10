package cats
package data

final case class NonEmptyList[A](head: A, tail: List[A]) {
  def map[B](f: A => B): NonEmptyList[B] = NonEmptyList(f(head), tail.map(f))
}

object NonEmptyList {
  def one[A](a: A): NonEmptyList[A] = NonEmptyList(a, Nil)
}
