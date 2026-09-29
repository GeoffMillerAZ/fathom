import Foundation
import Core
import struct Net.Client
import Missing

func run() {
    let m = Model()
    m.save()
    print(format(label(m)))
    Client().fetch()
}
