export const ChatStream = async (data) => {
    const response = await fetch('/api/kubernetes/chat/query', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'x-token': localStorage.getItem('token') || '',
        },
        body: JSON.stringify(data),
    });

    if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
    }

    if (!response.body) {
        throw new Error('response.body is not a ReadableStream');
    }

    return response.body.getReader();
};
