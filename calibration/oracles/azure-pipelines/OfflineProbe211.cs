// Authored controls only. Calls the existing pinned evaluator, never executes a job.
using System;
using System.Collections.Generic;
using System.Linq;
using System.Text.Json;
using System.Threading.Tasks;
using GitHub.DistributedTask.Expressions2;
using GitHub.DistributedTask.ObjectTemplating;
using Runner.Server.Azure.Devops;

class Probe211
{
    static async Task<object> Evaluate(string name, string stage, bool malformed = false, bool missing = false)
    {
        var root = "stages:\n- template: stage.yml\n  parameters:\n    name: " + stage + "\n- stage: Publish\n  dependsOn: " + stage + "\n  jobs:\n  - job: Release\n    steps:\n    - script: echo never-executed\n";
        var template = "parameters:\n- name: name\n  type: string\nstages:\n- stage: ${{ parameters.name }}\n  jobs:\n  - job: Compile\n    steps:\n    - script: echo never-executed\n";
        var files = new List<KeyValuePair<string, string>> {
            new("pipeline.yml", malformed ? "stages: [\n" : root)
        };
        if (!missing) files.Add(new("stage.yml", template));
        var context = new Context {
            FileProvider = new DefaultInMemoryFileProviderFileProvider(files.ToArray()),
            TraceWriter = new EmptyTraceWriter(),
            Flags = ExpressionFlags.DTExpressionsV1 | ExpressionFlags.ExtendedDirectives | ExpressionFlags.AllowAnyForInsert
        };
        Pipeline pipeline;
        try {
            var token = await AzureDevops.ReadTemplate(context, "pipeline.yml");
            pipeline = await new Pipeline().Parse(context.ChildContext(token, "pipeline.yml"), token);
            pipeline.CheckPipelineForRuntimeFailure();
        } catch (TemplateValidationException) {
            return new { @case = name, status = "refused", stages = Array.Empty<string>(), jobs = Array.Empty<string>(), dependencies = Array.Empty<string>() };
        }
        // Other exceptions remain a failed process, never an expected refusal.
        var stages = pipeline.Stages.Select(s => s.Name).ToArray();
        var jobs = pipeline.Stages.SelectMany(s => s.Jobs).Select(j => j.Name).ToArray();
        var deps = pipeline.Stages.SelectMany(s => (s.DependsOn ?? Array.Empty<string>()).Select(d => s.Name + ":" + d)).ToArray();
        return new { @case = name, status = "evaluated", stages, jobs, dependencies = deps };
    }
    static async Task Main()
    {
        var cases = new[] {
            await Evaluate("template", "Build"), await Evaluate("changed", "Changed"),
            await Evaluate("malformed", "Build", malformed: true),
            await Evaluate("missing", "Build", missing: true)
        };
        Console.WriteLine(JsonSerializer.Serialize(new {
            classification = "feasibility-only", support_claim = false, native_owner_attested = false, cases
        }));
    }
}
