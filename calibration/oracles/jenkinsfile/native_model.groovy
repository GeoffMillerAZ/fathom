// Bench-only owner model enumeration; never executes a subject pipeline.
// Jenkins pipeline-model-definition 2.2293.v6e7193cec599, source 6e7193cec599b402ee79ee9afc5b8a417eb43967.
import groovy.json.JsonOutput
import java.lang.reflect.Modifier
import java.security.MessageDigest
import java.nio.file.Files
import jenkins.model.Jenkins

new Thread({
try {
    while(Jenkins.get().getInitLevel()!=hudson.init.InitMilestone.COMPLETED)Thread.sleep(50)
    def jenkins=Jenkins.get()
    def loader=jenkins.pluginManager.uberClassLoader
    def converter=loader.loadClass('org.jenkinsci.plugins.pipeline.modeldefinition.parser.Converter')
    def element=loader.loadClass('org.jenkinsci.plugins.pipeline.modeldefinition.ast.ModelASTElement')
    def input=new File(System.getenv('FATHOM_NATIVE_INPUTS')).canonicalFile
    def output=new File(System.getenv('FATHOM_NATIVE_RAW'))
    def toolCount=0
    def toolPath=System.getenv('FATHOM_TOOL_DECLARATIONS')
    if(toolPath) {
        def declarations=new groovy.json.JsonSlurper().parse(new File(toolPath))
        // Owner-reported tool names register descriptors only. No tool is installed or run.
        def placeholder=new File(jenkins.rootDir,'declaration-only-tool').path
        def mavens=declarations.findAll { it.type=='maven' }.collect { new hudson.tasks.Maven.MavenInstallation(it.name,placeholder,[]) }
        if(mavens)jenkins.getDescriptorByType(hudson.tasks.Maven.DescriptorImpl).setInstallations(mavens as hudson.tasks.Maven.MavenInstallation[])
        def jdks=declarations.findAll { it.type=='jdk' }.collect { new hudson.model.JDK(it.name,placeholder) }
        if(jdks)jenkins.setJDKs(jdks)
        toolCount=declarations.size()
    }
    def files=[]
    input.eachFileRecurse { file -> if(file.isFile())files.add(file) }
    files.sort { a,b -> a.path<=>b.path }
    if(files.size()>128)throw new IllegalArgumentException('input count cap')
    def records=[]
    long bytes=0
    files.each { file ->
        byte[] data=Files.readAllBytes(file.toPath())
        bytes+=data.length
        if(data.length>1048576 || bytes>8388608)throw new IllegalArgumentException('source byte cap')
        def relative=input.toPath().relativize(file.toPath())
        def subject=relative.nameCount>1?relative.getName(0).toString():'authored'
        def row=[subject:subject,sha256:MessageDigest.getInstance('SHA-256').digest(data).encodeHex().toString(),status:'refused',nodes:[]]
        try {
            def model=converter.getMethod('scriptToPipelineDef',String.class).invoke(null,new String(data,'UTF-8'))
            if(model==null)row.status='no_pipeline'
            else {
                def seen=Collections.newSetFromMap(new IdentityHashMap())
                def walk
                walk={ value,int parent ->
                    if(value==null)return
                    if(element.isInstance(value)) {
                        if(!seen.add(value))return
                        if(row.nodes.size()>=100000)throw new IllegalArgumentException('node cap')
                        def source=value.getSourceLocation()
                        int line=source==null?-1:source.getLineNumber()
                        int end=source==null?-1:source.getLastLineNumber()
                        def node=[type:value.class.name.tokenize('.').last(),name:null,line:line,end:end,parent:parent]
                        int index=row.nodes.size();row.nodes.add(node)
                        def fields=[]
                        for(def type=value.class;type!=null && element.isAssignableFrom(type);type=type.superclass) {
                            fields.addAll(type.declaredFields.findAll { !Modifier.isStatic(it.modifiers) && !it.synthetic && it.name!='sourceLocation' })
                        }
                        fields.sort { a,b -> (a.declaringClass.name+'.'+a.name)<=>(b.declaringClass.name+'.'+b.name) }
                        fields.each { field ->
                            field.accessible=true
                            def child=field.get(value)
                            if(field.name=='name' && child instanceof String)node.name=child
                            walk(child,index)
                        }
                    } else if(value instanceof Map) {
                        value.each { key,child -> walk(key,parent);walk(child,parent) }
                    } else if(value instanceof Iterable) {
                        value.each { child -> walk(child,parent) }
                    }
                }
                walk(model,-1)
                row.status='parsed'
            }
        } catch(Exception error) {
            // Parser detail stays in the private controller log, never the receipt.
            error.printStackTrace()
            row.status='refused';row.nodes=[]
        }
        records.add(row)
    }
    output.text=JsonOutput.toJson([core_version:Jenkins.VERSION,tool_declarations:toolCount,files:records])+'\n'
} finally {
    // No HTTP connector, job, build, pipeline execution or background controller.
    Jenkins.get().cleanUp()
    System.exit(0)
}
} as Runnable,'native-model-audit').start()
