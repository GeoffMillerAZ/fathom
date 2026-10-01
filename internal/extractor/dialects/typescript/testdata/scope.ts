// scope.ts — which bindings are definitions (LB-Q-typescript-rest): a
// module-scope binding, an exported declaration (a namespace's too), a class
// method and a declared function (at any depth) are; a function-local arrow
// const or function expression, a test-callback helper, a namespace-local
// binding and an object-literal method or pair are not, and a call written
// in one belongs to the definition around it.
export const parse = <T>(input: string): T => decode<T>(input);
const internalArrow = async (n: number): Promise<number> => await compute(n);
var legacy = function (x: number): number {
  return twice(x);
};

export function handle(req: Request): Response {
  const toBody = (r: Request) => serialize(r);
  const fallback = function (): Response {
    return empty();
  };
  function nested(): void {
    audit();
  }
  nested();
  return toBody(req) ?? fallback();
}

export class Service {
  private readonly cache = new Map<string, string>();

  lookup(key: string): string | undefined {
    const miss = () => fetchRemote(key);
    return this.cache.get(key) ?? miss();
  }
}

export namespace Codec {
  export const encode = (value: unknown): string => stringify(value);
  const hiddenHelper = (s: string) => s.trim();
  export function decode(raw: string): unknown {
    return hiddenHelper(raw);
  }
}

export const handlers = {
  create(input: string) {
    return persist(input);
  },
  remove: (id: string) => drop(id),
};

export default {
  name: 'scope',
  setup() {
    return init();
  },
};

describe('handle', () => {
  const makeRequest = (): Request => build();
  it('answers', async () => {
    const check = function (res: Response): boolean {
      return verify(res);
    };
    check(handle(makeRequest()));
  });
});
