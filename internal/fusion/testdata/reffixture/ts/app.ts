import { User, Config, assist } from './index';
import * as m from './models';

export function make(c: Config): User {
  return new User();
}

export class Service {
  id: m.Id = '';

  run() {
    const local = 1;
    return [assist, local];
  }
}
