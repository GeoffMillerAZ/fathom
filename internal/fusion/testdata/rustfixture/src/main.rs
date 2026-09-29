mod net;
mod util;

use crate::util::helper;
use net::connect;

fn main() {
    helper();
    util::other();
    connect();
}
