// contracts: what the retired CGO rustextract.go pinned and the dialect keeps (LB-F390):
// each kind's name, an impl named by its type and its methods qualified by it, a module,
// and a call and a macro invocation spelled by the callee's name.
fn hello() -> u32 { 42 }
struct Foo { x: i32 }
enum Bar { A, B }
trait MyTrait { fn method(&self); }
mod mymod { }
impl Foo { fn new() -> Foo { Foo { x: 0 } } }
impl MyType { fn do_work(&self) {} fn helper() {} }

fn greet() {
    helper();
    println!("hi");
}
fn helper() {}
