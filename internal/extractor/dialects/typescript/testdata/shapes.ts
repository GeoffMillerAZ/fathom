import { Injectable } from './di';
import type { Config } from './config';
import * as util from '../util';

export interface Shape {
  area(): number;
  name: string;
}

export type Point = { x: number; y: number };
type Handler<T> = (value: T) => void;

export enum Color {
  Red,
  Green = 'green',
}

const enum Flag { A = 1 }

export abstract class Base<T> implements Shape {
  name = 'base';
  #priv = 0;

  constructor(private readonly cfg: Config) {
    util.log(cfg);
  }

  abstract area(): number;

  protected describe(
    prefix: string,
    suffix?: string,
  ): string {
    return format(prefix, this.name, suffix);
  }

  #secret(): void {
    this.describe('a');
  }
}

@Injectable()
export class Circle extends Base<number> {
  area(): number {
    return Math.PI * square(this.r);
  }
}

export function create<T>(kind: string): T {
  return factory.make<T>(kind);
}

export const arrow = async (n: number): Promise<number> => await compute(n);

namespace Geometry {
  export function inner() {
    return outer();
  }
}

declare function ambient(x: number): void;
