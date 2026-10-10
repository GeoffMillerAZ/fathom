package app

import cats.data.NonEmptyList
import cats.{Functor => F, _}
import cats.syntax.all._
import cats.data.{Validated, Valid => V}
import scala.collection.mutable

object Use {
  def go(): Unit = {
    NonEmptyList(1, Nil)
    V(2)
    showAll("x")
  }
}
