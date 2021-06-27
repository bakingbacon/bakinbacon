import React, { useCallback, useState, createContext } from 'react';
import Toast from 'react-bootstrap/Toast';

const ToasterContext = createContext();
export default ToasterContext

export const ToasterContextProvider = ({children}) => {

	const [toasts, setToasts] = useState([]);

	const addToast = useCallback(
		function(toast) {
			toast.id = Math.floor((Math.random() * 1001) + 1);
			toast.autohide = toast.autohide || 0;
			setToasts((toasts) => [...toasts, toast]);
		},
		[setToasts]
	);

	const deleteToast = (id) => {
		setToasts(toasts => toasts.filter(e => e.id !== id));
	};

	return (
		<ToasterContext.Provider value={addToast}>
			{children}
			<div className={"toasters-container"} >
				{ toasts.map((toast) => (
					<Toast
						key={toast.id}
						onClose={() => deleteToast(toast.id)}
						className={"toaster-"+toast.type}
						{... (toast.autohide > 0 ? {delay: toast.autohide, autohide: true} : {})}
					>
					  <Toast.Header>
						<strong className="mr-auto">{toast.title}</strong>
					  </Toast.Header>
					  <Toast.Body>{toast.msg}</Toast.Body>
					</Toast>
				  ))
				}
			</div>
		</ToasterContext.Provider>
	);
}
