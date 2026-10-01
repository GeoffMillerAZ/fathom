package com.acme.app

import com.acme.util.DEFAULT_NAME
import com.acme.util.Text
import com.acme.util.trimAll

fun run(): Text {
    return Text(trimAll(DEFAULT_NAME))
}
