// A syntax error the parser must survive.
fn before(a: f32) -> f32 {
  return a;
}

fn oops( {
  let = ;
}

fn after(b: f32) -> f32 {
  return b;
}

const Done: u32 = 1u;
