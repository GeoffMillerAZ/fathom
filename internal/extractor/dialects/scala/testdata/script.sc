import $ivy.`com.lihaoyi::os-lib:0.9.1`

val root = os.pwd

def list(dir: os.Path): Seq[os.Path] =
  os.list(dir)

case class Entry(path: String)

println(list(root))
