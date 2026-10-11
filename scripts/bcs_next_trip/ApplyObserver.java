import java.nio.file.Files;
import java.nio.file.Paths;
import jdk.internal.org.objectweb.asm.ClassReader;
import jdk.internal.org.objectweb.asm.ClassWriter;
import jdk.internal.org.objectweb.asm.Opcodes;
import jdk.internal.org.objectweb.asm.tree.AbstractInsnNode;
import jdk.internal.org.objectweb.asm.tree.ClassNode;
import jdk.internal.org.objectweb.asm.tree.InsnList;
import jdk.internal.org.objectweb.asm.tree.MethodInsnNode;
import jdk.internal.org.objectweb.asm.tree.MethodNode;
import jdk.internal.org.objectweb.asm.tree.VarInsnNode;

/** Build-time-only instrumenter. The user's JAR is not distributed. */
public final class ApplyObserver {
    public static void main(String[] args) throws Exception {
        ClassNode original = new ClassNode();
        new ClassReader(Files.readAllBytes(Paths.get(args[0]))).accept(original, 0);
        ClassNode template = new ClassNode();
        new ClassReader(Files.readAllBytes(Paths.get(args[1]))).accept(template, 0);
        if (!original.name.equals("bbs/f")) throw new Exception("Unexpected logger class");
        MethodNode observer = null, entry = null;
        for (MethodNode m : template.methods)
            if (m.name.equals("bridgeNextTripDetails")) observer = m;
        for (MethodNode m : original.methods) {
            if (m.name.equals("bridgeNextTripDetails")) throw new Exception("Already instrumented");
            if (m.name.equals("a") && m.desc.equals("(Ljava/lang/String;)V")) entry = m;
        }
        if (observer == null || entry == null) throw new Exception("Logger entry not found");
        // The template uses StringBuilder bytecode (--release 8) and no extra
        // class. ASM remaps its constant pool and retains all existing methods.
        observer.access = Opcodes.ACC_PRIVATE | Opcodes.ACC_STATIC;
        original.methods.add(observer);
        InsnList prefix = new InsnList();
        prefix.add(new VarInsnNode(Opcodes.ALOAD, 1));
        prefix.add(new MethodInsnNode(Opcodes.INVOKESTATIC, original.name,
            observer.name, observer.desc, false));
        prefix.add(new VarInsnNode(Opcodes.ASTORE, 1));
        entry.instructions.insert(prefix);
        ClassWriter writer = new ClassWriter(ClassWriter.COMPUTE_MAXS);
        original.accept(writer);
        Files.write(Paths.get(args[2]), writer.toByteArray());
    }
}
