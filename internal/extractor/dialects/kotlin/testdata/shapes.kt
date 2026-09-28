package com.example.shapes

import kotlin.math.PI
import com.example.util.Logger as Log

val topLevel = 10
var mutableTop: String = "x"

interface Shape {
    fun area(): Double
}

abstract class Base(val name: String) : Shape {
    private val cached = 0

    open fun describe(
        prefix: String,
        suffix: String = "",
    ): String {
        return format(prefix, name, suffix)
    }

    class Nested {
        fun deep() = compute()

        inner class Inner {
            fun deeper() {
                Log.info("x")
            }
        }
    }

    companion object Factory {
        fun create(): Base = Circle(1.0)
    }
}

data class Circle(val r: Double) : Base("circle") {
    override fun area(): Double = PI * square(r)
}

enum class Color { RED, GREEN }

sealed class Result {
    object Empty : Result()
}

object Registry {
    fun register(shape: Shape) {
        shapes.add(shape)
    }
}

fun String.shout(): String = this.uppercase()

suspend fun load(id: Int): String {
    return repository.find(id).await()
}

fun main() {
    val c = Circle(2.0)
    println(c.area())
    Registry.register(c)
}
