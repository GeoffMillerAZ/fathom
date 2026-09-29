// A syntax error the parser must survive: definitions either side still extract.

pub fn good() -> u32 {
    1
}

pub struct Holder {
    field: u32,
}

impl Holder {
    pub fn read(&self) -> u32 {
        self.field
    }

    fn broken( {
        let x = ;
    }

    pub fn after(&self) -> u32 {
        good()
    }
}

pub fn survivor(x: u32) -> u32 {
    good() + x
}
