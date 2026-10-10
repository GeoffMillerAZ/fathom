mod util;
mod net;
pub mod inline {
    pub fn inside() {}
}
use crate::util::Helper;
use std::fmt;
use self::inline::inside as here;

pub fn top() {}
