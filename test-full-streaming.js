// Test script to verify full streaming functionality with search
const testFullStreaming = async () => {
    console.log('Testing full streaming pipeline with search...\n');
    
    const query = 'artificial intelligence';
    const url = 'http://localhost:8080/api/v1/search';
    
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'text/event-stream'
            },
            body: JSON.stringify({ 
                query: query,
                stream: true,
                safe_search: true,
                num_results: 3
            })
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        console.log('✅ Full streaming request initiated successfully');
        console.log('📡 Receiving streaming data:\n');
        
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let tokenCount = 0;
        let searchResults = [];
        
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
                        
                        if (data.type === 'search_results') {
                            searchResults = data.results || [];
                            console.log(`🔍 Found ${searchResults.length} search results`);
                        } else if (data.type === 'search_complete') {
                            console.log('✅ Search phase completed, starting LLM processing...');
                        } else if (data.type === 'token' && data.token) {
                            tokenCount++;
                            process.stdout.write(`[${tokenCount}] ${data.token} `);
                        } else if (data.type === 'summary' && data.summary) {
                            console.log(`\n\n📝 Final summary: ${data.summary}`);
                        } else if (data.type === 'complete') {
                            console.log(`\n\n✅ Process completed`);
                        } else if (data.type === 'error') {
                            console.log(`\n❌ Error: ${data.message}`);
                        } else if (data.message) {
                            console.log(`\n💬 ${data.message}`);
                        }
                    } catch (e) {
                        // Ignore malformed JSON
                        console.log(`\n⚠️  Malformed data: ${line.slice(6)}`);
                    }
                }
            }
        }
        
        console.log(`\n\n📊 Results:`);
        console.log(`   Search results: ${searchResults.length}`);
        console.log(`   Streaming tokens: ${tokenCount}`);
        
        if (tokenCount > 0) {
            console.log('🎉 Cleaned up streaming implementation working correctly!');
        } else {
            console.log('⚠️  No tokens received - may be using non-streaming mode or error occurred');
        }
        
    } catch (error) {
        console.error('❌ Streaming test failed:', error.message);
    }
};

// Run the test
testFullStreaming();