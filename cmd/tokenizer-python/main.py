#!/usr/bin/env python3
"""
Python Tokenizer Service
Handles both tokenization and detokenization for BART models
Mac-optimized with proper Hugging Face tokenizers
"""

import asyncio
import logging
import signal
import sys
import time
import os
from typing import Optional

import grpc
from transformers import AutoTokenizer
import redis

# Import generated protobuf code
sys.path.append('/app/proto')
sys.path.append('proto')
import proto.search_pb2 as pb2
import proto.search_pb2_grpc as pb2_grpc

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.StreamHandler(sys.stdout),
        logging.FileHandler('/tmp/tokenizer.log') if os.path.exists('/tmp') else logging.NullHandler()
    ]
)
logger = logging.getLogger(__name__)


class TokenizerService(pb2_grpc.TokenizerServiceServicer):
    """
    Python-based tokenizer service with real BART tokenization
    Supports both tokenization and detokenization
    """
    
    def __init__(self):
        self.tokenizers = {}
        self.redis_client = None
        self._initialize_redis()
        self._initialize_tokenizers()
    
    def _initialize_redis(self):
        """Initialize Redis connection for caching"""
        try:
            redis_host = os.getenv('REDIS_HOST', 'localhost')
            self.redis_client = redis.Redis(host=redis_host, port=6379, db=0, decode_responses=True)
            self.redis_client.ping()
            logger.info(f"Connected to Redis at {redis_host}")
        except Exception as e:
            logger.warning(f"Redis connection failed, continuing without cache: {e}")
            self.redis_client = None
    
    def _initialize_tokenizers(self):
        """Initialize supported tokenizers"""
        models = [
            "facebook/bart-large-cnn",
            "google-t5/t5-base",
            "microsoft/DialoGPT-small"
        ]
        
        for model in models:
            try:
                logger.info(f"Loading tokenizer: {model}")
                self.tokenizers[model] = AutoTokenizer.from_pretrained(model, use_fast=True)
                vocab_size = len(self.tokenizers[model])
                logger.info(f"✅ {model} loaded - vocab size: {vocab_size}")
            except Exception as e:
                logger.error(f"Failed to load {model}: {e}")
        
        # Set default tokenizer
        if "facebook/bart-large-cnn" in self.tokenizers:
            self.default_tokenizer = self.tokenizers["facebook/bart-large-cnn"]
            self.default_model = "facebook/bart-large-cnn"
        else:
            self.default_tokenizer = list(self.tokenizers.values())[0]
            self.default_model = list(self.tokenizers.keys())[0]
        
        logger.info(f"Default tokenizer: {self.default_model}")
    
    def _get_tokenizer(self, model_name: str):
        """Get tokenizer for specified model or return default"""
        return self.tokenizers.get(model_name, self.default_tokenizer)
    
    def _cache_key(self, prefix: str, text_or_tokens: str, model_name: str, **kwargs) -> str:
        """Generate cache key"""
        import hashlib
        content = f"{text_or_tokens}|{model_name}|{kwargs}"
        hash_obj = hashlib.md5(content.encode())
        return f"{prefix}:{hash_obj.hexdigest()[:16]}"
    
    def Tokenize(self, request, context):
        """Tokenize text into token IDs"""
        start_time = time.time()
        
        try:
            logger.info(f"Tokenizing text for model '{request.model_name}', length: {len(request.text)} chars")
            
            # Get tokenizer
            tokenizer = self._get_tokenizer(request.model_name)
            actual_model = request.model_name if request.model_name in self.tokenizers else self.default_model
            
            # Check cache
            cache_key = self._cache_key("tokenize", request.text, actual_model, 
                                      max_tokens=request.max_tokens, special=request.include_special_tokens)
            cache_status = "miss"
            
            if self.redis_client:
                try:
                    cached = self.redis_client.get(cache_key)
                    if cached:
                        import json
                        cached_data = json.loads(cached)
                        cache_status = "hit"
                        logger.info(f"Cache hit for tokenization")
                        
                        return pb2.TokenizeResponse(
                            token_ids=cached_data["token_ids"],
                            token_strings=cached_data["token_strings"],
                            token_count=len(cached_data["token_ids"]),
                            truncated_text=cached_data.get("truncated_text", ""),
                            was_truncated=cached_data.get("was_truncated", False),
                            model_used=actual_model,
                            processing_time_ms=cached_data.get("processing_time_ms", 0),
                            cache_status=cache_status,
                            success=True
                        )
                except Exception as e:
                    logger.warning(f"Cache read error: {e}")
            
            # Tokenize
            max_length = min(request.max_tokens, 1024) if request.max_tokens > 0 else 1024
            
            encoding = tokenizer(
                request.text,
                max_length=max_length,
                truncation=True,
                return_tensors=None,
                add_special_tokens=request.include_special_tokens
            )
            
            token_ids = encoding['input_ids']
            token_strings = tokenizer.convert_ids_to_tokens(token_ids)
            
            # Handle truncation
            was_truncated = len(token_ids) >= max_length
            truncated_text = request.text if not was_truncated else tokenizer.decode(token_ids, skip_special_tokens=True)
            
            processing_time = (time.time() - start_time) * 1000
            
            # Cache result
            if self.redis_client and cache_status == "miss":
                try:
                    import json
                    cache_data = {
                        "token_ids": token_ids,
                        "token_strings": token_strings,
                        "truncated_text": truncated_text,
                        "was_truncated": was_truncated,
                        "processing_time_ms": processing_time
                    }
                    self.redis_client.setex(cache_key, 3600, json.dumps(cache_data))  # 1 hour TTL
                except Exception as e:
                    logger.warning(f"Cache write error: {e}")
            
            logger.info(f"✅ Tokenization complete: {len(token_ids)} tokens ({processing_time:.2f}ms)")
            
            return pb2.TokenizeResponse(
                token_ids=token_ids,
                token_strings=token_strings,
                token_count=len(token_ids),
                truncated_text=truncated_text,
                was_truncated=was_truncated,
                model_used=actual_model,
                processing_time_ms=processing_time,
                cache_status=cache_status,
                success=True
            )
            
        except Exception as e:
            logger.error(f"Tokenization failed: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Tokenization failed: {str(e)}")
            
            return pb2.TokenizeResponse(
                success=False,
                error=str(e)
            )
    
    def Detokenize(self, request, context):
        """Convert token IDs back to text"""
        start_time = time.time()
        
        try:
            logger.info(f"Detokenizing {len(request.token_ids)} tokens for model '{request.model_name}'")
            
            # Get tokenizer
            tokenizer = self._get_tokenizer(request.model_name)
            actual_model = request.model_name if request.model_name in self.tokenizers else self.default_model
            
            # Check cache
            cache_key = self._cache_key("detokenize", str(request.token_ids), actual_model, 
                                      skip_special=request.skip_special_tokens)
            cache_status = "miss"
            
            if self.redis_client:
                try:
                    cached = self.redis_client.get(cache_key)
                    if cached:
                        import json
                        cached_data = json.loads(cached)
                        cache_status = "hit"
                        logger.info(f"Cache hit for detokenization")
                        
                        return pb2.DetokenizeResponse(
                            text=cached_data["text"],
                            token_count=len(request.token_ids),
                            model_used=actual_model,
                            processing_time_ms=cached_data.get("processing_time_ms", 0),
                            cache_status=cache_status,
                            success=True
                        )
                except Exception as e:
                    logger.warning(f"Cache read error: {e}")
            
            # Detokenize
            text = tokenizer.decode(
                request.token_ids, 
                skip_special_tokens=request.skip_special_tokens,
                clean_up_tokenization_spaces=True
            )
            
            processing_time = (time.time() - start_time) * 1000
            
            # Cache result
            if self.redis_client and cache_status == "miss":
                try:
                    import json
                    cache_data = {
                        "text": text,
                        "processing_time_ms": processing_time
                    }
                    self.redis_client.setex(cache_key, 3600, json.dumps(cache_data))  # 1 hour TTL
                except Exception as e:
                    logger.warning(f"Cache write error: {e}")
            
            logger.info(f"✅ Detokenization complete: {len(text)} chars ({processing_time:.2f}ms)")
            
            return pb2.DetokenizeResponse(
                text=text,
                token_count=len(request.token_ids),
                model_used=actual_model,
                processing_time_ms=processing_time,
                cache_status=cache_status,
                success=True
            )
            
        except Exception as e:
            logger.error(f"Detokenization failed: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Detokenization failed: {str(e)}")
            
            return pb2.DetokenizeResponse(
                success=False,
                error=str(e)
            )
    
    def BatchTokenize(self, request, context):
        """Batch tokenization (simple implementation)"""
        responses = []
        for req in request.requests:
            resp = self.Tokenize(req, context)
            responses.append(resp)
        
        return pb2.BatchTokenizeResponse(
            responses=responses,
            total_processing_time_ms=sum(r.processing_time_ms for r in responses),
            cache_hits=sum(1 for r in responses if r.cache_status == "hit"),
            cache_misses=sum(1 for r in responses if r.cache_status == "miss")
        )
    
    def BatchDetokenize(self, request, context):
        """Batch detokenization (simple implementation)"""  
        responses = []
        for req in request.requests:
            resp = self.Detokenize(req, context)
            responses.append(resp)
        
        return pb2.BatchDetokenizeResponse(
            responses=responses,
            total_processing_time_ms=sum(r.processing_time_ms for r in responses),
            cache_hits=sum(1 for r in responses if r.cache_status == "hit"),
            cache_misses=sum(1 for r in responses if r.cache_status == "miss")
        )
    
    def GetVocabularyInfo(self, request, context):
        """Get vocabulary information for a model"""
        try:
            tokenizer = self._get_tokenizer(request.model_name)
            actual_model = request.model_name if request.model_name in self.tokenizers else self.default_model
            
            vocab_size = len(tokenizer)
            special_tokens = []
            
            # Get special tokens
            if hasattr(tokenizer, 'special_tokens_map'):
                special_tokens = list(tokenizer.special_tokens_map.values())
            
            return pb2.VocabularyInfoResponse(
                vocab_size=vocab_size,
                special_tokens=special_tokens,
                encoding_name=tokenizer.__class__.__name__,
                model_name=actual_model
            )
            
        except Exception as e:
            logger.error(f"Failed to get vocabulary info: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Failed to get vocabulary info: {str(e)}")
            
            return pb2.VocabularyInfoResponse()
    
    def HealthCheck(self, request, context):
        """Health check endpoint"""
        status = "healthy"
        
        # Check if we have at least one tokenizer loaded
        if not self.tokenizers:
            status = "unhealthy"
        
        # Check Redis connection
        if self.redis_client:
            try:
                self.redis_client.ping()
            except:
                status = "degraded"  # Can work without Redis
        
        return pb2.HealthCheckResponse(
            status=status,
            service="tokenizer-python",
            timestamp=int(time.time())
        )


async def serve():
    """Start the gRPC server"""
    server = grpc.aio.server()
    
    try:
        # Initialize and register service
        tokenizer_service = TokenizerService()
        pb2_grpc.add_TokenizerServiceServicer_to_server(tokenizer_service, server)
        
        # Configure server
        listen_addr = '[::]:8082'
        server.add_insecure_port(listen_addr)
        
        logger.info("🚀 Python Tokenizer Service starting on port 8082")
        logger.info("Features: Real BART tokenization, Caching, Mac optimized")
        await server.start()
        
        # Graceful shutdown handler
        async def shutdown():
            logger.info("Shutting down tokenizer service...")
            await server.stop(grace=5)
            logger.info("Tokenizer service shutdown complete")
        
        # Register signal handlers
        loop = asyncio.get_event_loop()
        for sig in [signal.SIGINT, signal.SIGTERM]:
            loop.add_signal_handler(sig, lambda: asyncio.create_task(shutdown()))
        
        # Keep server running
        await server.wait_for_termination()
        
    except Exception as e:
        logger.error(f"Server startup failed: {e}")
        raise


if __name__ == "__main__":
    try:
        asyncio.run(serve())
    except KeyboardInterrupt:
        logger.info("Received interrupt signal")
    except Exception as e:
        logger.error(f"Server error: {e}")
        sys.exit(1)