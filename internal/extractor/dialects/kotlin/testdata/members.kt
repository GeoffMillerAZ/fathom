package com.example.members

import com.example.shapes.Shape
import com.example.util.*

typealias Handler = (String) -> Unit

const val LIMIT = 10
lateinit var late: String
val lazyOne: Int by lazy { compute() }
val String.lastChar: Char
    get() = this[length - 1]

annotation class Marker(val value: String = "")

fun interface Visitor {
    fun visit(n: Int)
}

sealed interface Node

@JvmInline
value class Id(val raw: Long)

enum class Level(val code: Int) {
    LOW(1) {
        override fun label() = "low"
    },
    HIGH(2);

    open fun label(): String = name
}

class User(val name: String, var age: Int = 0) : Node {
    val upper: String get() = name.uppercase()
    var counter = 0
        private set

    constructor(other: User) : this(other.name, other.age) {
        helper()
    }

    constructor(
        first: String,
        last: String,
    ) : this("$first $last")

    init {
        setup()
    }

    fun <T : Comparable<T>> sorted(
        items: List<T>,
        cmp: Comparator<T>? = null,
    ): List<T> where T : Any {
        val local = items.map { it.toString() }
        fun inner() = other()
        return items.sortedWith(cmp ?: naturalOrder())
    }

    operator fun plus(o: User): User = User(name + o.name, age + o.age)

    fun `does a thing`() {}

    companion object {
        const val TAG = "user"
        fun create() = User("a")
    }
}

object Holder : Shape {
    val instance = build()
    override fun area(): Double = 0.0
}

class Cfg(val first: Int, val second: Int = first)
