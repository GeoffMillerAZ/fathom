// Every call shape, and what is not a call.

fn helper(x: u32) -> u32 {
    x + 1
}

fn plain() -> u32 {
    helper(1)
}

fn qualified() {
    let a = crate::util::build();
    let b = self::local();
    let c = super::parent::lift(a);
    std::mem::drop(b);
    let d = Widget::default();
    drop((c, d));
}

fn methods(v: Vec<u32>) -> usize {
    let n = v.iter().map(|x| helper(*x)).count();
    v.len() + n
}

fn turbofish() {
    let parsed = "1".parse::<u32>().unwrap();
    let seq = collect::<Vec<u8>>();
    let cap = Vec::<u8>::with_capacity(4);
    let conv = <u32 as From<u8>>::from(3);
    drop((parsed, seq, cap, conv));
}

fn constructors() -> Option<Wrapper> {
    let w = Wrapper(1);
    let r: Result<u32, ()> = Ok(2);
    let e = Shape::Circle(1.0);
    let _ = (r, e);
    Some(w)
}

fn closures() {
    let f = |x: u32| helper(x) + other(x);
    let g = move || f(1);
    g();
}

struct Counter;

impl Counter {
    fn new() -> Self {
        Self::init()
    }

    fn init() -> Self {
        Counter
    }

    fn bump(&self) {
        self.step();
        Self::init();
        Counter::init();
    }

    fn step(&self) {}
}

async fn awaits() {
    fetch("u").await;
    client().get("x").send().await;
}

fn macros(items: &[u32]) {
    println!("{}", items.len());
    assert_eq!(helper(2), 3, "{}", label());
    let v = vec![make(); 3];
    let s = format!("{}-{}", first(&v), (1 + 2));
    my_macro!(alpha(1), beta.gamma(2));
    log::info!("done {}", s);
    if v.is_empty() {
        unreachable!();
    }
}

fn unsafe_call() {
    unsafe {
        ffi_entry(1);
    }
}
