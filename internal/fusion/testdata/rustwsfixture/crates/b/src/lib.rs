use a::{S};
use a::T;
use std::fmt;
use serde::{Serialize};

pub fn build(t: T, s: S) -> T {
    let _ = s;
    t
}
