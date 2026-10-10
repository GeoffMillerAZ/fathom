export class Box<T> {
  constructor(private value: T) {}
  map<U>(f: (v: T) => U): Box<U> {
    return new Box<U>(f(this.value));
  }
}

export function first<T extends object>(xs: Array<T>): T | undefined {
  const cast = <Record<string, T>>(<unknown>xs);
  return xs.length < 1 ? undefined : identity<T>(xs[0]) ?? cast['0'];
}

export function identity<T>(x: T): T {
  return x;
}

export interface Settings {
  name: string;
}

export type OptionsProps = { verbose: boolean };
