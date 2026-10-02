mod inner;
mod x;

pub use inner::T;

pub struct S;

#[cfg(test)]
mod tests {
    use super::*;
}
