package routes

import "testing"

func TestReadNest(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		src    string
		routes []string
		mounts []string
	}{
		{
			name: "controller prefix and method decorators",
			path: "src/cats/cats.controller.ts",
			src: `import { Controller, Get, Post, Put, Delete, Patch, All, Head, Options, Param, Body } from '@nestjs/common';

@Controller('cats')
export class CatsController {
  constructor(private readonly catsService: CatsService) {}

  @Post()
  @HttpCode(204)
  async create(@Body() dto: CreateCatDto): Promise<void> {
    this.catsService.create(dto);
  }

  @Get()
  findAll(@Query() query: ListAllEntities) { return []; }

  @Get(':id')
  findOne(@Param('id', new ParseIntPipe()) id: number) { return { id }; }

  @Put(':id') update(@Param('id') id: string) {}
  @Delete(':id') remove() {}
  @Patch('/:id/name/') rename() {}
  @All('wild') any() {}
  @Head() head() {}
  @Options('*') opts() {}
  private helper() { return 'not a route'; }
}
`,
			routes: []string{
				"ANY /cats/wild CatsController.any",
				"DELETE /cats/:id CatsController.remove",
				"GET /cats CatsController.findAll",
				"GET /cats/:id CatsController.findOne",
				"HEAD /cats CatsController.head",
				"OPTIONS /cats/* CatsController.opts",
				"PATCH /cats/:id/name CatsController.rename",
				"POST /cats CatsController.create",
				"PUT /cats/:id CatsController.update",
			},
		},
		{
			name: "no prefix, array paths, an options object, URI versions",
			path: "src/app.controller.ts",
			src: `import { Controller, Get, Version, VERSION_NEUTRAL } from '@nestjs/common';
const BASE = 'health';

@Controller()
export class AppController {
  @Get() root() {}
  @Get([BASE, 'ping']) both() {}
}

@Controller({ path: 'users', version: '1' })
export class UsersController {
  @Get() list() {}
  @Version('2')
  @Get(':id') one() {}
  @Version(['3', VERSION_NEUTRAL])
  @Get('x') x() {}
}

@Controller({ version: VERSION_NEUTRAL, path: ['a', 'b'] })
export class NeutralController {
  @Get('n') n() {}
}
`,
			routes: []string{
				"GET / AppController.root",
				"GET /a/n NeutralController.n",
				"GET /b/n NeutralController.n",
				"GET /health AppController.both",
				"GET /ping AppController.both",
				"GET /users/x UsersController.x",
				"GET /v1/users UsersController.list",
				"GET /v2/users/:id UsersController.one",
				"GET /v3/users/x UsersController.x",
			},
		},
		{
			name: "an unresolvable path is skipped, never guessed",
			path: "src/x.controller.ts",
			src: `@Controller(PREFIX)
export class XController { @Get() a() {} }
@Controller('y')
export class YController { @Get(dynamicPath) b() {} @Get('ok') c() {} }
`,
			routes: []string{"GET /y/ok YController.c"},
		},
		{
			name: "a module mounts its controllers and imported modules; RouterModule paths",
			path: "src/app.module.ts",
			src: `import { Module, forwardRef } from '@nestjs/common';
import { RouterModule } from '@nestjs/core';
import { ConfigModule } from '@nestjs/config';
import { CatsModule } from './cats/cats.module';
import { AdminModule } from './admin/admin.module';
import { MetricsModule } from './admin/metrics/metrics.module';
import { DogsModule } from './dogs/dogs.module';
import { AppController } from './app.controller';

@Module({
  imports: [
    ConfigModule.forRoot(),
    CatsModule,
    AdminModule,
    MetricsModule.register({}),
    forwardRef(() => DogsModule),
    RouterModule.register([
      { path: 'admin', module: AdminModule, children: [{ path: 'metrics', module: MetricsModule }] },
    ]),
  ],
  controllers: [AppController],
})
export class AppModule {}
`,
			mounts: []string{
				" -> ./app.controller",
				" -> ./cats/cats.module",
				" -> ./dogs/dogs.module",
				"/admin -> ./admin/admin.module",
				"/admin/metrics -> ./admin/metrics/metrics.module",
			},
		},
		{
			name: "setGlobalPrefix mounts the root module",
			path: "src/main.ts",
			src: `import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module.js';

async function bootstrap() {
  const app = await NestFactory.create<NestExpressApplication>(AppModule);
  app.setGlobalPrefix('api/', { exclude: ['health'] });
  await app.listen(3000);
}
bootstrap();
`,
			mounts: []string{"/api -> ./app.module"},
		},
		{
			name: "setGlobalPrefix with a root module that is not a repo import mounts the source directory",
			path: "apps/api/src/main.ts",
			src: `import { NestFactory } from '@nestjs/core';
import { AppModule } from '@app/core';
const app = await NestFactory.create(AppModule);
app.setGlobalPrefix('v1');
`,
			mounts: []string{"/v1 -> ./"},
		},
		{
			name: "a controller extending a mixin call",
			path: "src/m.controller.ts",
			src: `@Controller('m')
export class MController extends WithAuth(Base) implements OnModuleInit {
  @Get() m() {}
}
`,
			routes: []string{"GET /m MController.m"},
		},
		{
			name: "a class without @Controller is not a controller",
			path: "src/svc.ts",
			src: `@Injectable()
export class Svc { @Get('x') nope() {} }
`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkResult(t, readNest(c.path, []byte(c.src)), c.routes, c.mounts)
		})
	}
}
