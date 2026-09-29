package com.example.app

import com.example.util.Helper
import com.example.util.Strings as S
import com.example.other.*
import kotlin.collections.List

class App {
    fun run() {
        Helper.go()
        S.trim("x")
        val xs: List<Int> = other()
    }
}
