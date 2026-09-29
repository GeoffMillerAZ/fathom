package com.example.calls

fun calls(list: List<String>, maybe: Foo?, fn: () -> Unit) {
    plain(1)
    obj.method(2)
    a.b.c.deep(3)
    maybe?.safe(4)
    maybe!!.bang(5)
    generic<Int>(6)
    obj.gen<String>(7)
    Outer.Inner(8)
    list.map { it.trim() }.filter { keep(it) }
    fn()()
    foo { bar() }
    val ref = obj::refd
    val v = build(1).next().last
    arr[0].call()
}

class Multi : Base() {
    fun chain() = builder()
        .first()
        .second(1)

    fun own() {
        this.self()
        super.parent()
        val f = fun(x: Int) { inAnon(x) }
        f(1)
    }

    val handler = { later() }
}
