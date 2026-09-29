//! Every definition kind the dialect emits, and the shapes around them.

use std::fmt;

pub const MAX_RETRIES: u32 = 3;
static GREETING: &str = "hi";

pub type Result<T> = std::result::Result<T, Error>;

#[macro_export]
macro_rules! square {
    ($x:expr) => {
        $x * $x
    };
}

pub mod net {
    pub const PORT: u16 = 8080;

    pub fn connect(host: &str) -> bool {
        !host.is_empty()
    }

    pub mod tcp {
        pub struct Conn {
            pub fd: i32,
        }

        pub fn open() -> Conn {
            Conn { fd: 0 }
        }
    }
}

mod util;

pub struct Unit;

pub struct Pair(pub i32, pub i32);

#[derive(Debug, Clone)]
pub struct Widget<T>
where
    T: Clone,
{
    pub id: u32,
    pub payload: T,
}

pub enum Shape {
    Circle(f64),
    Square { side: f64 },
    Empty,
}

pub union Bits {
    int: u32,
    float: f32,
}

pub trait Render {
    type Output;
    const DEPTH: u32;

    fn render(&self) -> String;

    fn describe(&self) -> String {
        format!("{}", self.render())
    }
}

impl<T: Clone> Widget<T> {
    pub const LIMIT: u32 = 10;

    pub fn new(id: u32, payload: T) -> Self {
        Widget { id, payload }
    }

    pub fn with_payload<U>(
        &self,
        other: U,
    ) -> Widget<U>
    where
        U: Clone,
    {
        Widget { id: self.id, payload: other }
    }
}

impl fmt::Display for Unit {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "unit")
    }
}

impl<T> Render for Vec<T> {
    type Output = Vec<u8>;
    const DEPTH: u32 = 1;

    fn render(&self) -> String {
        String::new()
    }
}

impl Render for &str {
    type Output = String;
    const DEPTH: u32 = 0;

    fn render(&self) -> String {
        (*self).to_string()
    }
}

impl Render for [u8] {
    type Output = ();
    const DEPTH: u32 = 0;

    fn render(&self) -> String {
        String::new()
    }
}

impl net::tcp::Conn {
    pub fn close(self) {}
}

pub async fn fetch(url: &str) -> Result<String> {
    Ok(url.to_string())
}

pub const fn zero() -> u32 {
    0
}

pub(crate) unsafe extern "C" fn ffi_entry(x: i32) -> i32 {
    x
}

extern "C" {
    fn abs(x: i32) -> i32;
}

fn outer() {
    fn inner() {}
    struct Local;
    const SCRATCH: u32 = 1;
    inner();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn works() {
        assert!(zero() == 0);
    }
}
