// members: fields, enum variants and associated items (LB-F82).
pub struct Config {
    pub name: String,
    verbose: bool,
}

pub union Bits {
    a: u32,
    b: f32,
}

pub enum Shape {
    Empty,
    Circle(f64),
    Rect { w: u32, h: u32 },
}

pub trait Source {
    const LIMIT: usize;
    type Item;
    fn next(&mut self) -> Option<Self::Item>;
}

impl Source for Config {
    const LIMIT: usize = 4;
    type Item = u8;
    fn next(&mut self) -> Option<u8> {
        None
    }
}

impl Config {
    const DEFAULT: u32 = 1;
}
