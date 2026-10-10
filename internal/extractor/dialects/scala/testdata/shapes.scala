package com.example
package shapes

import scala.collection.mutable

/** A shape. */
sealed trait Shape {
  def area: Double
  val sides: Int
  var label: String
  type Unit <: AnyRef
}

case class Circle(radius: Double, name: String = "circle") extends Shape {
  def area: Double = math.Pi * radius * radius
  val sides = 0
  var label = "round"
}

final class Square(val side: Double, var tag: String, hidden: Int) extends Shape {
  def this(side: Double) = this(side, "", 0)

  override def area: Double =
    side * side

  lazy val diagonal: Double = math.sqrt(2) * side
  private[this] val secret = hidden
  val sides, corners = 4
  var label = "boxy"
  type Unit = String

  def +(other: Square): Square = new Square(side + other.side)
  def `weird name`(): Int = 1
}

case object Origin

object Shapes {
  final val Max = 10
  implicit val ordering: Ordering[Double] = Ordering.Double

  class Registry[A](initial: List[A]) {
    def all: List[A] = initial
  }

  object Nested {
    def deep(n: Int): Int = n
  }

  def largest(xs: List[Shape]): Option[Shape] =
    if (xs.isEmpty) None
    else Some(xs.maxBy(_.area))
}

package object shapes {
  type Area = Double
  def unit: Area = 1.0
}

abstract class Base[F[_]](implicit val F: Functor[F])
