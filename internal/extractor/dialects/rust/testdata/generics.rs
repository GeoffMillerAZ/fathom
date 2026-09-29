// Generics, lifetimes and trait objects; impls inside modules and functions.

pub mod parse {
    use std::str::FromStr;

    pub struct Token<'a> {
        pub text: &'a str,
    }

    impl<'a> Token<'a> {
        pub fn new(text: &'a str) -> Token<'a> {
            Token { text }
        }

        pub fn len(&self) -> usize {
            self.text.len()
        }
    }

    impl<'a> Iterator for Token<'a> {
        type Item = char;

        fn next(&mut self) -> Option<char> {
            self.text.chars().next()
        }
    }

    impl FromStr for Token<'static> {
        type Err = ();

        fn from_str(s: &str) -> Result<Self, ()> {
            Err(())
        }
    }
}

pub trait Visitor<T>: Send
where
    T: Clone,
{
    fn visit(&mut self, node: &T) -> bool;

    fn visit_all<I>(&mut self, nodes: I) -> bool
    where
        I: IntoIterator<Item = T>,
    {
        nodes.into_iter().all(|n| self.visit(&n))
    }
}

impl<T: Clone> Visitor<T> for Vec<T> {
    fn visit(&mut self, node: &T) -> bool {
        self.push(node.clone());
        true
    }
}

impl Visitor<u8> for Box<dyn Fn(u8) -> bool> {
    fn visit(&mut self, node: &u8) -> bool {
        (self)(*node)
    }
}

pub struct Wrapper<T>(pub T);

impl<T> From<T> for Wrapper<T> {
    fn from(t: T) -> Self {
        Wrapper(t)
    }
}

pub fn make_adder(n: u32) -> impl Fn(u32) -> u32 {
    move |x| x + n
}

pub fn boxed() -> Box<dyn Fn(u32) -> u32> {
    Box::new(make_adder(1))
}

fn with_local_impl() {
    struct Point;

    impl Point {
        fn origin() -> Point {
            Point
        }
    }

    let _ = Point::origin();
}

pub struct Plain;

impl<'a> Visitor<u8> for &'a [u8] {
    fn visit(&mut self, node: &u8) -> bool {
        self.len() > *node as usize
    }
}

impl Visitor<u8> for &&Plain {
    fn visit(&mut self, node: &u8) -> bool {
        *node > 0
    }
}
