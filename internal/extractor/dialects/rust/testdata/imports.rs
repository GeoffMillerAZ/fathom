// Every use and mod form the import query reads.
mod child;
mod inline_mod {}
use a;
use a::b::c;
use a::b::c as d;
pub use crate::x::y;
use ::std::io;
use self::m::n;
use super::sup;
use a::b::*;
use a::{b, c as d2, e::f, self};
use {p::q, r};
use crate::deep::{self, item};
use crate::flags::{lowargs::{BinaryMode, Mode as M}, hiargs::*, parse::{inner::{self, leaf}}};

pub fn entry() -> u32 {
    c() + item()
}
