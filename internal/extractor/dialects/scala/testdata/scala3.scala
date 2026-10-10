package scala3

enum Color(val rgb: Int):
  case Red extends Color(0xff0000)
  case Green, Blue

  def hex: String = rgb.toHexString

enum Tree[+A]:
  case Leaf(value: A)
  case Node(left: Tree[A], right: Tree[A])

opaque type Meters = Double

def topLevel(n: Int): Int = n + 1

val topValue: Int = topLevel(1)

trait Show[A]:
  def show(a: A): String

object Instances:
  given Show[Int] with
    def show(a: Int): String = a.toString

  given stringShow: Show[String] with
    def show(a: String): String = a

  given listOrd[T](using ord: Ordering[T]): Ordering[List[T]] = ???

  given Ordering[Int] = Ordering.Int

  extension (s: String)
    def shout: String = s.toUpperCase
    def whisper: String = s.toLowerCase

  extension [A](xs: List[A]) def second: A = xs.tail.head
end Instances

class Box[A](val value: A):
  def map[B](f: A => B): Box[B] =
    Box(f(value))
end Box
