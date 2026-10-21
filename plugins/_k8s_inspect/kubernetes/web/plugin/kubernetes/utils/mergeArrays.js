export function extractNameValuePairs(obj) {
    let result = [];

    // Iterate through each key in the object
    for (let key in obj) {
        if (Array.isArray(obj[key])) {
            obj[key].forEach(item => {
                if (item.name && item.value !== undefined) {
                    result.push({ name: item.name, value: item.value });
                }

                // If value is an object, we need to go deeper
                if (typeof item.value === 'object' && item.value !== null) {
                    result = result.concat(extractNameValuePairs(item.value));
                }
            });
        } else if (typeof obj[key] === 'object' && obj[key] !== null) {
            result = result.concat(extractNameValuePairs(obj[key]));
        }
    }

    return result;
}
