export  function renderTraceData(trace) {
    const threadInfo = trace.root;
    const timestamp = new Date(threadInfo.timestamp);
    const shanghaiOffset = 8 * 60 * 60 * 1000; // UTC+8 in milliseconds

    // Adjust to Shanghai time
    const shanghaiTime = new Date(timestamp.getTime() + shanghaiOffset);
    const formattedTimestamp = shanghaiTime.toISOString().replace('T', ' ').substring(0, 19); // Format to "YYYY-MM-DD HH:MM:SS"

    const threadName = threadInfo.threadName;
    const daemon = threadInfo.daemon;
    const priority = threadInfo.priority;
    const threadId = threadInfo.threadId;
    const classloader = threadInfo.classloader;

    // Construct the first line
    let result = `---ts=${formattedTimestamp};thread_name=${threadName};id=${threadId};is_daemon=${daemon};priority=${priority};TCCL=${classloader}</br>`;

    // Recursive function to render method calls
    function renderMethod(method, totalCost, indent = '') {
        const percent = ((method.cost / totalCost) * 100).toFixed(2);
        const cost = (method.cost / 1000000).toFixed(6) + 's';
        const lineNumber = method.lineNumber !== -1 ? `#${method.lineNumber}` : '';
        const methodName = `${method.className}:${method.methodName}`;

        let methodString = `${indent}----[${percent}% ${cost}] ${methodName} ${lineNumber}</br>`;

        // If there are children, recurse into them
        if (method.children && method.children.length > 0) {
            const childIndent = indent + '    '.replace(/ /g, '&nbsp;') + '&nbsp;&nbsp;&nbsp;&nbsp;'; // Increase indentation for children by adding extra &nbsp;
            method.children.forEach(child => {
                methodString += renderMethod(child, totalCost, childIndent);
            });
        }

        return methodString;
    }

    // Process the root method trace
    const rootMethod = threadInfo.children[0];
    const totalCost = rootMethod.totalCost;

    result += `---[${(totalCost / 1000000).toFixed(6)}s] ${rootMethod.className}:${rootMethod.methodName}()</br>`;

    rootMethod.children.forEach(child => {
        result += renderMethod(child, totalCost, '    '.replace(/ /g, '&nbsp;')); // Add initial indentation for first level children
    });

    // Add a final line break
    return result + '</br></br>';
}
