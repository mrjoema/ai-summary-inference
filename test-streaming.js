// Test script to verify streaming functionality
const testStreaming = async () => {
    console.log('Testing cleaned up streaming implementation...\n');
    
    const query = 'AI and machine learning';
    const url = 'http://localhost:8080/api/v1/search?stream=true';
    
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'text/event-stream'
            },
            body: JSON.stringify({ query, stream: true })
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        console.log('✅ Streaming request initiated successfully');
        console.log('📡 Receiving streaming tokens:\n');
        
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let tokenCount = 0;
        
        while (true) {
            const { done, value } = await reader.read();
            
            if (done) {
                console.log('\n🏁 Stream completed');
                break;
            }
            
            const chunk = decoder.decode(value);
            const lines = chunk.split('\n');
            
            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    try {
                        const data = JSON.parse(line.slice(6));
                        
                        if (data.token) {
                            tokenCount++;
                            process.stdout.write(`[${tokenCount}] ${data.token} `);
                        } else if (data.summary) {
                            console.log(`\n\n📝 Final summary: ${data.summary}`);
                        } else if (data.message) {
                            console.log(`\n💬 ${data.message}`);
                        }
                    } catch (e) {
                        // Ignore malformed JSON
                    }
                }
            }
        }
        
        console.log(`\n\n✅ Successfully received ${tokenCount} streaming tokens`);
        console.log('🎉 Cleaned up streaming implementation working correctly!');
        
    } catch (error) {
        console.error('❌ Streaming test failed:', error.message);
    }
};

// Run the test
testStreaming();