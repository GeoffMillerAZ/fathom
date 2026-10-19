package Outer {
  sub first { 1 }

  package Inner::Deep {
    sub second {
      return 2;
    }
  }

  sub third { 3 }
}

package Plain::After 1.5;

sub fourth { 4 }
