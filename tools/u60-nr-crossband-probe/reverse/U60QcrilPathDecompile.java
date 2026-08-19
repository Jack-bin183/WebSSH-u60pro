import ghidra.app.decompiler.DecompInterface;
import ghidra.app.decompiler.DecompileResults;
import ghidra.app.script.GhidraScript;
import ghidra.program.model.address.Address;
import ghidra.program.model.listing.Function;
import ghidra.program.model.listing.FunctionIterator;

/**
 * Analysis-only helper for recovering the two QCRIL control paths used by the
 * cross-band probe. It is not built into or deployed with the device tool.
 */
public class U60QcrilPathDecompile extends GhidraScript {
    private final String[] needles = {
        "start_advanced_scan",
        "perform_incremental_network_scan",
        "handleStartNetworkScan",
        "fill_band_info",
        "request_set_system_selection_channels",
        "handleSetSystemSelectionChannels",
        "dispatchStartNetworkScanRequest",
        "dispatchSetSystemSelectionChannels"
    };

    @Override
    public void run() throws Exception {
        DecompInterface decompiler = new DecompInterface();
        decompiler.openProgram(currentProgram);
        FunctionIterator functions = currentProgram.getFunctionManager().getFunctions(true);
        while (functions.hasNext()) {
            Function function = functions.next();
            String name = function.getName();
            boolean match = false;
            for (String needle : needles) {
                if (name.contains(needle)) {
                    match = true;
                    break;
                }
            }
            if (!match) {
                continue;
            }
            println("=== " + name + " @ " + function.getEntryPoint() + " ===");
            DecompileResults result = decompiler.decompileFunction(function, 180, monitor);
            if (result.decompileCompleted()) {
                println(result.getDecompiledFunction().getC());
            } else {
                println("decompile failed: " + result.getErrorMessage());
            }
        }
        // Stripped helper called by the ONE_SHOT path. Its address is stable
        // for the U60 firmware image documented in VALIDATION-U60.md.
        Address builderAddress = toAddr(0x3953d0L);
        Function builder = currentProgram.getFunctionManager().getFunctionAt(builderAddress);
        if (builder != null) {
            println("=== one_shot_request_builder @ " + builder.getEntryPoint() + " ===");
            DecompileResults result = decompiler.decompileFunction(builder, 180, monitor);
            if (result.decompileCompleted()) {
                println(result.getDecompiledFunction().getC());
            } else {
                println("decompile failed: " + result.getErrorMessage());
            }
        }
        decompiler.dispose();
    }
}
