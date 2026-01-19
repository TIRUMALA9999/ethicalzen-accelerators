"""
Flask Integration Example

This example shows how to integrate EthicalZen guardrails
into a Flask API that wraps an LLM.

Run:
    pip install flask requests
    ETHICALZEN_API_KEY=sk-... python app.py
"""

import os
import requests
from functools import wraps
from flask import Flask, request, jsonify

app = Flask(__name__)

# EthicalZen API configuration
ETHICALZEN_API_URL = "https://api.ethicalzen.ai/v1/evaluate"
ETHICALZEN_API_KEY = os.environ.get("ETHICALZEN_API_KEY", "")


def evaluate_guardrail(guardrail: str, input_text: str) -> dict:
    """Call EthicalZen API to evaluate content against a guardrail."""
    response = requests.post(
        ETHICALZEN_API_URL,
        headers={
            "Authorization": f"Bearer {ETHICALZEN_API_KEY}",
            "Content-Type": "application/json"
        },
        json={
            "guardrail": guardrail,
            "input": input_text
        }
    )
    return response.json()


def guardrail_protected(guardrails: list):
    """
    Decorator to protect an endpoint with guardrails.
    
    Usage:
        @app.route('/chat', methods=['POST'])
        @guardrail_protected(['pii_blocker', 'prompt_injection_blocker'])
        def chat():
            ...
    """
    def decorator(f):
        @wraps(f)
        def decorated_function(*args, **kwargs):
            # Extract user input from request
            data = request.get_json() or {}
            user_input = data.get('message') or data.get('prompt') or ''
            
            if not user_input:
                return f(*args, **kwargs)
            
            # Check against all guardrails
            for guardrail in guardrails:
                try:
                    result = evaluate_guardrail(guardrail, user_input)
                    
                    if result.get('decision') == 'block':
                        return jsonify({
                            'error': 'Content blocked by safety guardrail',
                            'guardrail': guardrail,
                            'reason': result.get('reason', 'Policy violation')
                        }), 400
                        
                except Exception as e:
                    app.logger.error(f"Guardrail check failed: {e}")
                    # Fail-closed: block on error
                    return jsonify({
                        'error': 'Safety check failed',
                        'message': 'Unable to verify content safety'
                    }), 500
            
            return f(*args, **kwargs)
        return decorated_function
    return decorator


@app.route('/chat', methods=['POST'])
@guardrail_protected([
    'prompt_injection_blocker',
    'pii_blocker', 
    'medical_advice_smart'
])
def chat():
    """Chat endpoint protected by guardrails."""
    data = request.get_json()
    message = data.get('message', '')
    
    # Your LLM call here (OpenAI, Anthropic, etc.)
    llm_response = call_your_llm(message)
    
    # Optional: Check the response too
    output_check = evaluate_guardrail('medical_advice_smart', llm_response)
    
    if output_check.get('decision') == 'block':
        return jsonify({
            'response': "I'm not able to provide that specific advice. Please consult a professional."
        })
    
    return jsonify({'response': llm_response})


def call_your_llm(message: str) -> str:
    """Replace with your actual LLM call."""
    return f"This is a response to: {message}"


@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint."""
    return jsonify({'status': 'healthy'})


if __name__ == '__main__':
    if not ETHICALZEN_API_KEY:
        print("Warning: ETHICALZEN_API_KEY not set")
    
    print("Starting server...")
    print("POST /chat with { 'message': 'your message' }")
    app.run(host='0.0.0.0', port=5000, debug=True)

